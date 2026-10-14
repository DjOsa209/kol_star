package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIPConversationStreamDeliversBeforeModelFinishes(t *testing.T) {
	previewReceived := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input map[string]any
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		if input["stream"] != true || r.Header.Get("Accept") != "text/event-stream" || r.Header.Get("Authorization") != "Bearer test" {
			t.Error("missing streaming request or server-side authentication")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		chunk := func(text string) {
			body, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]string{"content": text}}}})
			_, _ = fmt.Fprintf(w, "data: %s\r\n\r\n", body)
			w.(http.Flusher).Flush()
		}
		chunk(`{"presentation":{"headline":"先评估设`)
		select {
		case <-previewReceived:
		case <-time.After(time.Second):
			t.Error("answer was buffered until the model finished")
		}
		chunk(`计资源","explanation":"优先产品适配","basis":[{"label":"授权","value":"已获授权"}],"primaryResourceId":999,"alternativeResourceId":7},"criteria":{"markets":["东南亚"]}}`)
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer provider.Close()
	previews := []ipRecommendationPresentation{}
	plan, err := (&app{}).requestIPConversationPlanResponse(context.Background(), assistantAIModel{BaseURL: provider.URL, APIKey: "test", Model: "mock", Timeout: 2 * time.Second}, recommendationSubscription{}, "比较设计方向", "compare", []map[string]any{{"resourceId": int64(7)}}, []int64{7}, true, func(p ipRecommendationPresentation) error {
		previews = append(previews, p)
		if len(previews) == 1 {
			close(previewReceived)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(previews) < 2 || previews[0].Headline != "先评估设" || plan.Presentation.Headline != "先评估设计资源" {
		t.Fatalf("incremental headline was lost: %+v", previews)
	}
	if plan.Presentation.PrimaryResourceID != 0 || plan.Presentation.AlternativeResourceID != 7 || len(plan.Presentation.Basis) != 0 {
		t.Fatal("stream bypassed candidate or unsupported-claim validation")
	}
}

func TestIPPartialPresentationHandlesEscapesAndCorrectObject(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{`{"criteria":{"query":"headline伪字段"},"presentation":{"headline":"设计`, "设计"},
		{`{"presentation":{"headline":"设计\"表达\n第二行`, "设计\"表达\n第二行"},
		{`{"presentation":{"headline":"设计\`, "设计"},
		{`{"presentation":{"headline":"设计\u4`, "设计"},
		{`{"presentation":{"headline":"设计\ud83d`, "设计"},
		{`{"presentation":{"headline":"设计\ud83d\ude80`, "设计🚀"},
	} {
		p, ok := partialIPPresentation(test.input, nil)
		if !ok || p.Headline != test.want {
			t.Fatalf("%s => %q, want %q", test.input, p.Headline, test.want)
		}
	}
	if _, ok := partialIPPresentation(`{"criteria":{"headline":"不可展示"}`, nil); ok {
		t.Fatal("a criteria field became an assistant headline")
	}
}

func TestIPChatCompletionStreamRejectsTruncationAndProviderErrors(t *testing.T) {
	for _, body := range []string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"半段回答\"}}]}\n\n",
		"data: {\"error\":{\"message\":\"provider error\"}}\n\n",
		"data: {\"choices\":[{\"finish_reason\":\"length\"}]}\n\n",
		"data: {invalid}\n\n",
	} {
		if _, err := readIPChatCompletionStream(strings.NewReader(body), nil); err == nil {
			t.Fatalf("accepted incomplete/error stream: %s", body)
		}
	}
}

func TestIPConversationStreamFlushesThroughLoggingWriter(t *testing.T) {
	continueAnswer := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logged := &loggingResponseWriter{ResponseWriter: w}
		stream := newIPConversationStream(r.Context(), logged)
		defer stream.close()
		if err := stream.send("status", map[string]string{"message": "正在生成"}); err != nil {
			t.Error(err)
			return
		}
		select {
		case <-continueAnswer:
		case <-time.After(time.Second):
			t.Error("middleware buffered status")
		}
		if err := stream.send("done", apiResponse{Code: 0}); err != nil {
			t.Error(err)
		}
		if !logged.apiResponseOK() {
			t.Error("stream completion was recorded as failed")
		}
	}))
	defer server.Close()
	response, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.Header.Get("X-Accel-Buffering") != "no" || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatal("proxy buffering protection missing")
	}
	line, err := bufio.NewReader(response.Body).ReadString('\n')
	if err != nil || line != "event: status\n" {
		t.Fatalf("no live status: %q, %v", line, err)
	}
	close(continueAnswer)
}

func TestIPConversationStreamCancellationReachesProvider(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	providerCanceled := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"presentation\\\":{\\\"headline\\\":\\\"设计\"}}]}\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
			close(providerCanceled)
		case <-time.After(2 * time.Second):
		}
	}))
	defer provider.Close()
	defer cancel()
	_, err := (&app{}).requestIPConversationPlanResponse(ctx, assistantAIModel{BaseURL: provider.URL, Model: "mock", Timeout: 2 * time.Second}, recommendationSubscription{}, "继续", "clarify", nil, nil, true, func(ipRecommendationPresentation) error { cancel(); return ctx.Err() })
	if err == nil {
		t.Fatal("canceled answer reported success")
	}
	select {
	case <-providerCanceled:
	case <-time.After(time.Second):
		t.Fatal("provider request continued after cancellation")
	}
}

func TestIPConversationStreamAuthenticatesBeforeOpeningSSE(t *testing.T) {
	response := httptest.NewRecorder()
	(&app{}).ipRecommendationConversation(response, httptest.NewRequest(http.MethodPost, "/business/recommendations/ip/conversation", strings.NewReader(`{"action":"clarify","message":"找设计IP","stream":true}`)))
	if response.Code != http.StatusUnauthorized || strings.HasPrefix(response.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatal("unauthenticated client received a streaming connection")
	}
}
