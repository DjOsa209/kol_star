package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// Only public TikTok image nodes are accepted; redirects are disabled by the client.
func validMonthlyCoverURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" {
		return false
	}
	host := u.Hostname()
	return (host == "p16-common-sign.tiktokcdn.com" || host == "p19-common-sign.tiktokcdn.com") && !strings.Contains(u.Path, "..")
}
func monthlyCoverReferences(body []byte, id string) ([]string, error) {
	z := html.NewTokenizer(bytes.NewReader(body))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return nil, errors.New("cover metadata unavailable")
		case html.StartTagToken:
			token := z.Token()
			if token.Data != "script" {
				continue
			}
			found := false
			for _, a := range token.Attr {
				if a.Key == "id" && a.Val == "__FRONTITY_CONNECT_STATE__" {
					found = true
				}
			}
			if !found || z.Next() != html.TextToken {
				continue
			}
			var state struct {
				Source struct {
					Data map[string]struct {
						VideoData struct {
							ItemInfos struct {
								Covers []string `json:"covers"`
								Origin []string `json:"coversOrigin"`
							} `json:"itemInfos"`
						} `json:"videoData"`
					} `json:"data"`
				} `json:"source"`
			}
			if json.Unmarshal(z.Text(), &state) != nil {
				return nil, errors.New("invalid cover metadata")
			}
			item := state.Source.Data["/embed/v2/"+id].VideoData.ItemInfos
			urls := []string{}
			for _, u := range append(item.Covers, item.Origin...) {
				if validMonthlyCoverURL(u) {
					urls = append(urls, u)
				}
			}
			if len(urls) == 0 {
				return nil, errors.New("cover reference missing")
			}
			return urls, nil
		}
	}
}
func monthlyPublicBytes(ctx context.Context, client *http.Client, address string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	res, err := client.Do(req)
	if err != nil {
		return nil, errors.New("cover upstream unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New("cover upstream rejected")
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil || int64(len(b)) > limit {
		return nil, errors.New("cover response invalid")
	}
	return b, nil
}
func fetchMonthlyCover(ctx context.Context, client *http.Client, id string) error {
	if !trendIDPattern.MatchString(id) {
		return errors.New("invalid video ID")
	}
	body, err := monthlyPublicBytes(ctx, client, "https://www.tiktok.com/embed/v2/"+id, monthlyMaxBytes)
	if err != nil {
		return err
	}
	urls, err := monthlyCoverReferences(body, id)
	if err != nil {
		return err
	}
	for i, u := range urls {
		if i >= 4 || ctx.Err() != nil {
			break
		}
		imageCtx, imageCancel := context.WithTimeout(ctx, 4*time.Second)
		b, e := monthlyPublicBytes(imageCtx, client, u, 5<<20)
		imageCancel()
		if e != nil {
			continue
		}
		cfg, _, e := image.DecodeConfig(bytes.NewReader(b))
		if e != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4096 || cfg.Height > 4096 || cfg.Width*cfg.Height > 12000000 {
			continue
		}
		img, _, e := image.Decode(bytes.NewReader(b))
		if e != nil {
			continue
		}
		dir := filepath.Join(trendDataDir(), "media")
		if e = os.MkdirAll(dir, 0755); e != nil {
			return e
		}
		f, e := os.CreateTemp(dir, "monthly-cover-*.tmp")
		if e != nil {
			return e
		}
		e = jpeg.Encode(f, img, &jpeg.Options{Quality: 85})
		closeErr := f.Close()
		if e == nil {
			e = closeErr
		}
		if e == nil {
			e = os.Rename(f.Name(), filepath.Join(dir, "monthly_cover_"+id+".jpg"))
		}
		os.Remove(f.Name())
		return e
	}
	return errors.New("cover download failed")
}

type monthlyCoverResult struct {
	Ready   int `json:"ready"`
	Added   int `json:"added"`
	Pending int `json:"pending"`
}

func syncMonthlyCovers(ctx context.Context, client *http.Client, archive monthlyArchive, month string) monthlyCoverResult {
	result := monthlyCoverResult{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, 3)
	seen := map[string]bool{}
	for _, row := range archive.Records {
		if row.Month != month {
			continue
		}
		id := row.VideoID
		if !trendIDPattern.MatchString(id) {
			mu.Lock()
			result.Pending++
			mu.Unlock()
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		if st, e := os.Lstat(filepath.Join(trendDataDir(), "media", "monthly_cover_"+id+".jpg")); e == nil && st.Mode().IsRegular() {
			mu.Lock()
			result.Ready++
			mu.Unlock()
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
				mu.Lock()
				result.Pending++
				mu.Unlock()
				return
			}
			itemCtx, cancel := context.WithTimeout(ctx, 18*time.Second)
			defer cancel()
			err := fetchMonthlyCover(itemCtx, client, id)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				result.Added++
				result.Ready++
			} else {
				result.Pending++
			}
		}()
	}
	wg.Wait()
	return result
}
