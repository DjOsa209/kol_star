package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// The browser receives validated presentation snapshots, never raw model JSON.
// The terminal done event is sent only after the complete answer is persisted.
type ipConversationStream struct {
	w      http.ResponseWriter
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	stop   chan struct{}
}

func newIPConversationStream(ctx context.Context, w http.ResponseWriter) *ipConversationStream {
	ctx, cancel := context.WithCancel(ctx)
	s := &ipConversationStream{w: w, ctx: ctx, cancel: cancel, stop: make(chan struct{})}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = s.write(": keep-alive\n\n")
			case <-ctx.Done():
				return
			case <-s.stop:
				return
			}
		}
	}()
	return s
}

func (s *ipConversationStream) close() {
	close(s.stop)
	s.cancel()
	// Wait for any in-flight heartbeat before request logging inspects the writer.
	s.mu.Lock()
	s.mu.Unlock()
}

func (s *ipConversationStream) write(frame string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ctx.Err(); err != nil {
		return err
	}
	_, err := io.WriteString(s.w, frame)
	if err == nil {
		err = http.NewResponseController(s.w).Flush()
	}
	if err != nil {
		s.cancel()
	}
	return err
}

func (s *ipConversationStream) send(event string, data any) error {
	if ipLanguage(s.ctx) == "en" && (event == "status" || event == "error") {
		if message, ok := data.(map[string]string); ok {
			copy := map[string]string{}
			for k, v := range message {
				copy[k] = v
			}
			copy["message"] = ipEnglishText(copy["message"])
			data = copy
		}
	}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	err = s.write("event: " + event + "\ndata: " + string(body) + "\n\n")
	if err == nil && (event == "done" || event == "error") {
		s.mu.Lock()
		if recorder := ipLoggingWriter(s.w); recorder != nil {
			recorder.apiCode, recorder.hasAPICode = 0, true
			if event == "error" {
				recorder.apiCode = 500
			}
		}
		s.mu.Unlock()
	}
	return err
}

// ChatCompletion providers may split a JSON string or a Chinese character over
// network reads. Scanner assembles complete SSE lines; only choice 0 is used.
func readIPChatCompletionStream(body io.Reader, onContent func(string) error) (string, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var content strings.Builder
	data := []string{}
	dataSize := 0
	finished := false
	done := false
	dispatch := func() error {
		if len(data) == 0 {
			return nil
		}
		payload := strings.Join(data, "\n")
		data = nil
		dataSize = 0
		if strings.TrimSpace(payload) == "[DONE]" {
			done = true
			return nil
		}
		var chunk struct {
			Error   json.RawMessage `json:"error"`
			Choices []struct {
				Index int `json:"index"`
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return fmt.Errorf("模型流式片段格式不合法: %w", err)
		}
		if len(chunk.Error) > 0 && string(chunk.Error) != "null" {
			return errors.New("模型流式服务返回错误")
		}
		for _, choice := range chunk.Choices {
			if choice.Index != 0 {
				continue
			}
			if choice.FinishReason != nil {
				if *choice.FinishReason != "stop" {
					return errors.New("模型回答未完整生成")
				}
				finished = true
			}
			if choice.Delta.Content == "" {
				continue
			}
			if content.Len()+len(choice.Delta.Content) > 1<<20 {
				return errors.New("模型回答过长")
			}
			content.WriteString(choice.Delta.Content)
			if onContent != nil {
				if err := onContent(content.String()); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := dispatch(); err != nil {
				return "", err
			}
			if done {
				break
			}
		} else if value, ok := strings.CutPrefix(line, "data:"); ok {
			dataSize += len(value)
			if dataSize > 1<<20 {
				return "", errors.New("模型流式片段过长")
			}
			data = append(data, strings.TrimPrefix(value, " "))
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if err := dispatch(); err != nil {
		return "", err
	}
	if (!finished && !done) || content.Len() == 0 {
		return "", errors.New("模型流式连接提前结束")
	}
	return content.String(), nil
}

// Decode only fields reached in the actual presentation object. An unfinished
// headline/explanation is completed for preview; IDs and arrays wait for their
// full JSON values and are validated against the current candidate whitelist.
func partialIPPresentation(content string, candidates []map[string]any) (ipRecommendationPresentation, bool) {
	decoder := json.NewDecoder(strings.NewReader(content))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return ipRecommendationPresentation{}, false
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			break
		}
		if key != "presentation" {
			var ignored json.RawMessage
			if decoder.Decode(&ignored) != nil {
				break
			}
			continue
		}
		if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
			break
		}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				break
			}
			name, ok := key.(string)
			if !ok {
				break
			}
			offset := decoder.InputOffset()
			var value json.RawMessage
			if err := decoder.Decode(&value); err == nil {
				fields[name] = value
				continue
			}
			if name == "headline" || name == "explanation" {
				suffix := strings.TrimSpace(content[offset:])
				suffix = strings.TrimSpace(strings.TrimPrefix(suffix, ":"))
				if text, ok := partialIPJSONString(suffix); ok {
					fields[name], _ = json.Marshal(text)
				}
			}
			break
		}
		break
	}
	encoded, _ := json.Marshal(fields)
	var p ipRecommendationPresentation
	if json.Unmarshal(encoded, &p) != nil {
		return p, false
	}
	p = cleanIPPresentation(p, candidates)
	p.Source = "ai"
	p.Adjustments = []string{} // Only the server's actual changes are authoritative.
	return p, p.Headline != ""
}

func partialIPJSONString(value string) (string, bool) {
	if !strings.HasPrefix(value, `"`) {
		return "", false
	}
	var text string
	if json.Unmarshal([]byte(value), &text) == nil {
		return text, true
	}
	// At most twelve suffix bytes need removing: one incomplete Unicode escape
	// or a surrogate pair. Never expose a replacement character during preview.
	for removed := 0; removed <= 12 && len(value)-removed > 0; removed++ {
		candidate := value[:len(value)-removed] + `"`
		if json.Unmarshal([]byte(candidate), &text) == nil && !strings.HasSuffix(text, "\ufffd") {
			return text, true
		}
	}
	return "", false
}
