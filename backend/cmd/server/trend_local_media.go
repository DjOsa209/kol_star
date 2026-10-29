package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const localImageLimit int64 = 5 << 20

func validLocalImageURL(raw string) bool {
	if len(raw) > 4096 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	host := u.Hostname()
	for _, domain := range []string{"tiktokcdn.com", "tiktokcdn-us.com", "ibytedtos.com"} {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}
func publicTrendIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return false
	}
	for _, cidr := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32"} {
		if netip.MustParsePrefix(cidr).Contains(addr) {
			return false
		}
	}
	return true
}
func newLocalImageClient() *http.Client {
	transport := &http.Transport{MaxIdleConns: 8, MaxIdleConnsPerHost: 4, IdleConnTimeout: 30 * time.Second, TLSHandshakeTimeout: 8 * time.Second, ResponseHeaderTimeout: 10 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil || port != "443" {
			return nil, errors.New("invalid image endpoint")
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(ips) == 0 {
			return nil, errors.New("image host unavailable")
		}
		for _, ip := range ips {
			if !publicTrendIP(ip.IP) {
				return nil, errors.New("nonpublic image host")
			}
		}
		// Dial the already-validated address to avoid a second DNS lookup/rebinding.
		dialer := net.Dialer{Timeout: 8 * time.Second}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
	return &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func localImageContent(raw []byte) bool {
	if len(raw) < 12 || int64(len(raw)) > localImageLimit {
		return false
	}
	switch http.DetectContentType(raw) {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	}
	return false
}
func saveLocalTrendImages(ctx context.Context, root string, d *trendDataset) (int, int) {
	client := newLocalImageClient()
	defer client.CloseIdleConnections()
	return saveLocalTrendImagesUsing(ctx, root, d, client)
}
func saveLocalTrendImagesUsing(ctx context.Context, root string, d *trendDataset, client *http.Client) (int, int) {
	urls := map[string]bool{}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, value := range x {
				if k == "coverURL" || k == "avatarURL" {
					if raw, ok := value.(string); ok && raw != "" {
						urls[raw] = true
					}
				} else {
					walk(value)
				}
			}
		case []any:
			for _, value := range x {
				walk(value)
			}
		}
	}
	for _, row := range d.Rows {
		walk(row)
	}
	for _, detail := range d.Details {
		walk(detail)
	}
	replacements := map[string]string{}
	var lock sync.Mutex
	var downloaded atomic.Int64
	var failed atomic.Int64
	work := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for raw := range work {
				if !validLocalImageURL(raw) || ctx.Err() != nil || downloaded.Load() >= 256<<20 {
					failed.Add(1)
					continue
				}
				sum := sha256.Sum256([]byte(raw))
				name := hex.EncodeToString(sum[:]) + ".img"
				path := filepath.Join(root, name)
				validExisting := false
				if st, err := os.Lstat(path); err == nil && st.Mode().IsRegular() && st.Size() <= localImageLimit {
					if b, err := os.ReadFile(path); err == nil && localImageContent(b) {
						validExisting = true
					}
				}
				if !validExisting {
					request, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
					if err != nil {
						failed.Add(1)
						continue
					}
					response, err := client.Do(request)
					if err != nil {
						failed.Add(1)
						continue
					}
					body, readErr := io.ReadAll(io.LimitReader(response.Body, localImageLimit+1))
					response.Body.Close()
					totalBytes := downloaded.Add(int64(len(body)))
					if readErr != nil || response.StatusCode != 200 || !localImageContent(body) || totalBytes > 256<<20 {
						failed.Add(1)
						continue
					}
					if writeLocalTrendFile(root, name, body) != nil {
						failed.Add(1)
						continue
					}
				}
				lock.Lock()
				replacements[raw] = "/business/trends/local-media/" + name
				lock.Unlock()
			}
		}()
	}
	count := 0
	for raw := range urls {
		count++
		if count > 1500 {
			failed.Add(1)
			continue
		}
		select {
		case work <- raw:
		case <-ctx.Done():
			failed.Add(1)
		}
	}
	close(work)
	wg.Wait()
	var replace func(any)
	replace = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, value := range x {
				if k == "coverURL" || k == "avatarURL" {
					if raw, ok := value.(string); ok {
						if local, ok := replacements[raw]; ok {
							x[k] = local
						}
					}
				} else {
					replace(value)
				}
			}
		case []any:
			for _, value := range x {
				replace(value)
			}
		}
	}
	for _, row := range d.Rows {
		replace(row)
	}
	for _, detail := range d.Details {
		replace(detail)
	}
	return len(replacements), int(failed.Load())
}
