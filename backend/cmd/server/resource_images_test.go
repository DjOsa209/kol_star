package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestLocalizeResourceImageReplacesExistingVariant(t *testing.T) {
	previousRoot := resourceImageRoot
	previousClient := resourceImageHTTPClient.Load()
	resourceImageRoot = t.TempDir()
	t.Cleanup(func() {
		resourceImageRoot = previousRoot
		resourceImageHTTPClient.Store(previousClient)
	})

	resourceImageHTTPClient.Store(imageTestClient("image/png", []byte("png-image")))
	firstURL := localizeResourceImage(context.Background(), 42, "avatar", "https://example.com/avatar")
	if firstURL != "/api/uploads/resource-images/42/avatar.png" {
		t.Fatalf("unexpected first URL: %q", firstURL)
	}

	resourceImageHTTPClient.Store(imageTestClient("image/jpeg", []byte("jpeg-image")))
	secondURL := localizeResourceImage(context.Background(), 42, "avatar", "https://example.com/avatar")
	if secondURL != "/api/uploads/resource-images/42/avatar.jpg" {
		t.Fatalf("unexpected second URL: %q", secondURL)
	}

	resourceDir := filepath.Join(resourceImageRoot, "42")
	files, err := os.ReadDir(resourceDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name() != "avatar.jpg" {
		t.Fatalf("expected replacement file only, got %#v", files)
	}
	data, err := os.ReadFile(filepath.Join(resourceDir, "avatar.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "jpeg-image" {
		t.Fatalf("unexpected replacement data: %q", data)
	}
}

func TestLocalizeResourceImageUsesStablePostPath(t *testing.T) {
	previousRoot := resourceImageRoot
	previousClient := resourceImageHTTPClient.Load()
	resourceImageRoot = t.TempDir()
	t.Cleanup(func() {
		resourceImageRoot = previousRoot
		resourceImageHTTPClient.Store(previousClient)
	})

	resourceImageHTTPClient.Store(imageTestClient("image/webp", []byte("post-cover")))
	got := localizeResourceImage(context.Background(), 7, filepath.Join("posts", "TikTok_abc/123"), "https://example.com/cover")
	want := "/api/uploads/resource-images/7/posts/TikTok_abc/123.webp"
	if got != want {
		t.Fatalf("unexpected post image URL: got %q want %q", got, want)
	}
}

func TestLocalizeResourceImageIgnoresCanceledParentContext(t *testing.T) {
	previousRoot := resourceImageRoot
	previousClient := resourceImageHTTPClient.Load()
	resourceImageRoot = t.TempDir()
	t.Cleanup(func() {
		resourceImageRoot = previousRoot
		resourceImageHTTPClient.Store(previousClient)
	})

	resourceImageHTTPClient.Store(&http.Client{Transport: imageRoundTripper(func(req *http.Request) (*http.Response, error) {
		if err := req.Context().Err(); err != nil {
			t.Fatalf("download request used canceled context: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(bytes.NewReader([]byte("image-after-cancel"))),
		}, nil
	})})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := localizeResourceImage(ctx, 8, "avatar", "https://example.com/avatar")
	if got != "/api/uploads/resource-images/8/avatar.png" {
		t.Fatalf("unexpected image URL with canceled parent context: %q", got)
	}
}

func TestLocalizeResourceImageRetriesTemporaryDownloadFailure(t *testing.T) {
	previousRoot := resourceImageRoot
	previousClient := resourceImageHTTPClient.Load()
	resourceImageRoot = t.TempDir()
	t.Cleanup(func() {
		resourceImageRoot = previousRoot
		resourceImageHTTPClient.Store(previousClient)
	})

	var attempts atomic.Int32
	resourceImageHTTPClient.Store(&http.Client{Transport: imageRoundTripper(func(*http.Request) (*http.Response, error) {
		if attempts.Add(1) < 3 {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Status:     "503 Service Unavailable",
				Header:     http.Header{},
				Body:       io.NopCloser(bytes.NewReader(nil)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"image/jpeg"}},
			Body:       io.NopCloser(bytes.NewReader([]byte("retried-image"))),
		}, nil
	})})

	got := localizeResourceImage(context.Background(), 9, "avatar", "https://example.com/avatar")
	if got != "/api/uploads/resource-images/9/avatar.jpg" {
		t.Fatalf("unexpected retried image URL: %q", got)
	}
	if attempts.Load() != resourceImageDownloadAttempts {
		t.Fatalf("unexpected attempt count: %d", attempts.Load())
	}
}

type imageRoundTripper func(*http.Request) (*http.Response, error)

func (fn imageRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func imageTestClient(contentType string, body []byte) *http.Client {
	return &http.Client{Transport: imageRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{contentType}},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	})}
}
