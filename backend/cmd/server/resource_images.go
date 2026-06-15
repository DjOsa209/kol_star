package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const maxSyncedImageSize = 20 << 20
const resourceImageDownloadAttempts = 3

var resourceImageRoot = filepath.Join("uploads", "resource-images")
var resourceImageKeyPattern = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
var resourceImageHTTPClient atomic.Pointer[http.Client]

func init() {
	resourceImageHTTPClient.Store(newResourceImageHTTPClient(""))
}

func localizeResourceImage(ctx context.Context, resourceID int, key, sourceURL string) string {
	sourceURL = strings.TrimSpace(sourceURL)
	if resourceID <= 0 || sourceURL == "" || strings.HasPrefix(sourceURL, "/api/uploads/resource-images/") {
		return sourceURL
	}

	data, contentType, err := downloadResourceImage(ctx, sourceURL)
	if err != nil {
		log.Printf("download resource image failed resource=%d key=%s after %d attempts: %v", resourceID, key, resourceImageDownloadAttempts, err)
		return sourceURL
	}
	ext := syncedImageExt(contentType)
	if ext == "" {
		ext = syncedImageExt(http.DetectContentType(data))
	}
	if ext == "" {
		return sourceURL
	}

	cleanKey := sanitizeResourceImageKey(key)
	dir := filepath.Join(resourceImageRoot, fmt.Sprintf("%d", resourceID), filepath.Dir(cleanKey))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return sourceURL
	}
	base := filepath.Join(dir, filepath.Base(cleanKey))
	target := base + ext
	temp, err := os.CreateTemp(dir, ".sync-image-*")
	if err != nil {
		return sourceURL
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err = temp.Write(data); err != nil {
		_ = temp.Close()
		return sourceURL
	}
	if err = temp.Close(); err != nil {
		return sourceURL
	}
	removeResourceImageVariants(base, target)
	if err = os.Rename(tempName, target); err != nil {
		return sourceURL
	}

	relative, err := filepath.Rel(resourceImageRoot, target)
	if err != nil {
		return sourceURL
	}
	return "/api/uploads/resource-images/" + filepath.ToSlash(relative)
}

func newResourceImageHTTPClient(proxyURL string) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL = strings.TrimSpace(proxyURL); proxyURL != "" {
		if parsed, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(parsed)
		} else {
			log.Printf("resource image proxy ignored: %v", err)
		}
	}
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 20
	transport.IdleConnTimeout = 90 * time.Second
	transport.TLSHandshakeTimeout = 20 * time.Second
	transport.ResponseHeaderTimeout = 20 * time.Second
	return &http.Client{Transport: transport, Timeout: 45 * time.Second}
}

func configureResourceImageHTTPClient(proxyURL string) {
	resourceImageHTTPClient.Store(newResourceImageHTTPClient(proxyURL))
}

func downloadResourceImage(ctx context.Context, sourceURL string) ([]byte, string, error) {
	var lastErr error
	for attempt := 1; attempt <= resourceImageDownloadAttempts; attempt++ {
		data, contentType, retry, err := downloadResourceImageOnce(ctx, sourceURL)
		if err == nil {
			return data, contentType, nil
		}
		lastErr = err
		if !retry || attempt == resourceImageDownloadAttempts {
			break
		}
		timer := time.NewTimer(time.Duration(attempt) * 500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, "", ctx.Err()
		case <-timer.C:
		}
	}
	return nil, "", lastErr
}

func downloadResourceImageOnce(ctx context.Context, sourceURL string) ([]byte, string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, "", false, err
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; KOLAdminImageSync/1.0)")
	resp, err := resourceImageHTTPClient.Load().Do(req)
	if err != nil {
		return nil, "", retryableImageDownloadError(err), err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		retry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return nil, "", retry, fmt.Errorf("status=%s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSyncedImageSize+1))
	if err != nil {
		return nil, "", true, err
	}
	if len(data) == 0 {
		return nil, "", false, fmt.Errorf("empty image response")
	}
	if len(data) > maxSyncedImageSize {
		return nil, "", false, fmt.Errorf("image exceeds %s bytes", strconv.Itoa(maxSyncedImageSize))
	}
	contentType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	return data, contentType, false, nil
}

func retryableImageDownloadError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr)
}

func sanitizeResourceImageKey(key string) string {
	parts := strings.Split(filepath.ToSlash(strings.TrimSpace(key)), "/")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(resourceImageKeyPattern.ReplaceAllString(part, "_"), "._-")
		if part != "" {
			clean = append(clean, part)
		}
	}
	if len(clean) == 0 {
		return "image"
	}
	return filepath.Join(clean...)
}

func removeResourceImageVariants(base, keep string) {
	matches, _ := filepath.Glob(base + ".*")
	for _, match := range matches {
		if match != keep {
			_ = os.Remove(match)
		}
	}
}

func syncedImageExt(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/avif":
		return ".avif"
	default:
		return ""
	}
}
