package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// The narrow RESP2 adapter only implements cache GET and atomic SET PXAT.
// https://redis.io/docs/latest/commands/set/ (Redis >=6.2)
type RedisConfig struct {
	Addr       string `mapstructure:"addr"`
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	Database   int    `mapstructure:"database"`
	TLS        bool   `mapstructure:"tls"`
	KeyPrefix  string `mapstructure:"key_prefix"`
	NodeHour   int    `mapstructure:"node_hour"`
	NodeMinute int    `mapstructure:"node_minute"`
}

func (cfg RedisConfig) withDefaults() RedisConfig {
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:6379"
	}
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "kol:trends:v1:"
	}
	return cfg
}

type trendHotCache interface {
	Get(context.Context, string) ([]byte, error)
	Set(context.Context, string, []byte, time.Time) error
}

type redisTrendCache struct{ config RedisConfig }

const trendCacheMaxBytes = 8 << 20

func (c redisTrendCache) Get(ctx context.Context, key string) ([]byte, error) {
	return c.command(ctx, "GET", key)
}
func (c redisTrendCache) Set(ctx context.Context, key string, value []byte, expires time.Time) error {
	if len(value) > trendCacheMaxBytes {
		return errors.New("cache value too large")
	}
	res, err := c.command(ctx, "SET", key, string(value), "PXAT", strconv.FormatInt(expires.UnixMilli(), 10))
	if err == nil && string(res) != "OK" {
		return errors.New("cache write rejected")
	}
	return err
}

func (c redisTrendCache) command(ctx context.Context, args ...string) ([]byte, error) {
	cfg := c.config.withDefaults()
	host, _, err := net.SplitHostPort(cfg.Addr)
	if err != nil || cfg.Database < 0 || cfg.Database > 1023 {
		return nil, errors.New("invalid redis configuration")
	}
	ip := net.ParseIP(host)
	if !cfg.TLS && host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nil, errors.New("remote redis requires TLS")
	}
	ctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	dialer := &net.Dialer{Timeout: 800 * time.Millisecond}
	var conn net.Conn
	if cfg.TLS {
		d := &tls.Dialer{NetDialer: dialer, Config: &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}}
		conn, err = d.DialContext(ctx, "tcp", cfg.Addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", cfg.Addr)
	}
	if err != nil {
		return nil, errors.New("redis connection unavailable")
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	if err = conn.SetDeadline(deadline); err != nil {
		return nil, errors.New("redis deadline unavailable")
	}
	reader := bufio.NewReaderSize(conn, 4096)
	send := func(command ...string) ([]byte, error) {
		var frame bytes.Buffer
		fmt.Fprintf(&frame, "*%d\r\n", len(command))
		for _, arg := range command {
			fmt.Fprintf(&frame, "$%d\r\n", len(arg))
			frame.WriteString(arg)
			frame.WriteString("\r\n")
		}
		if _, err := io.Copy(conn, &frame); err != nil {
			return nil, errors.New("redis command unavailable")
		}
		return readTrendRESP(reader)
	}
	if cfg.Password != "" {
		auth := []string{"AUTH", cfg.Password}
		if cfg.Username != "" {
			auth = []string{"AUTH", cfg.Username, cfg.Password}
		}
		if res, err := send(auth...); err != nil || string(res) != "OK" {
			return nil, errors.New("redis authentication failed")
		}
	}
	if cfg.Database != 0 {
		if res, err := send("SELECT", strconv.Itoa(cfg.Database)); err != nil || string(res) != "OK" {
			return nil, errors.New("redis database unavailable")
		}
	}
	return send(args...)
}

func readTrendRESP(reader *bufio.Reader) ([]byte, error) {
	line, err := reader.ReadSlice('\n')
	if err != nil || len(line) < 3 || len(line) > 4096 || line[len(line)-2] != '\r' {
		return nil, errors.New("invalid redis response")
	}
	kind := line[0]
	value := line[1 : len(line)-2]
	switch kind {
	case '+':
		return append([]byte(nil), value...), nil
	case '-':
		return nil, errors.New("redis rejected cache command") // Never expose AUTH error content.
	case '$':
		n, err := strconv.Atoi(string(value))
		if err != nil || n < -1 || n > trendCacheMaxBytes {
			return nil, errors.New("invalid redis bulk size")
		}
		if n == -1 {
			return nil, nil
		}
		body := make([]byte, n+2)
		if _, err := io.ReadFull(reader, body); err != nil || body[n] != '\r' || body[n+1] != '\n' {
			return nil, errors.New("invalid redis bulk response")
		}
		return body[:n], nil
	default:
		return nil, errors.New("unsupported redis response")
	}
}

func trendCacheNode(now time.Time, cfg RedisConfig) (string, time.Time, error) {
	if cfg.NodeHour < 0 || cfg.NodeHour > 23 || cfg.NodeMinute < 0 || cfg.NodeMinute > 59 {
		return "", time.Time{}, errors.New("invalid cache reset node")
	}
	zone := time.FixedZone("Asia/Shanghai", 8*60*60)
	local := now.In(zone)
	node := time.Date(local.Year(), local.Month(), local.Day(), cfg.NodeHour, cfg.NodeMinute, 0, 0, zone)
	if !now.Before(node) {
		node = node.AddDate(0, 0, 1)
	}
	return node.AddDate(0, 0, -1).Format("20060102T1504"), node, nil
}

func trendHotKey(kind string, q trendQuery, language, bucket, prefix string) string {
	mode := "live"
	if q.archived() {
		mode = "archive"
	}
	canonical := fmt.Sprintf("%s|%s|%s|%d|%d|%d|%d|%s|%s|%s|%s", kind, q.Tab, q.Region, q.Period, q.Order, q.Page, q.Size, q.ID, mode, language, bucket)
	digest := sha256.Sum256([]byte(canonical))
	return prefix + bucket + ":" + hex.EncodeToString(digest[:])
}

type trendCacheRecord struct {
	ExpiresAt time.Time       `json:"expiresAt"`
	Body      json.RawMessage `json:"body"`
}
type trendCaptureWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *trendCaptureWriter) Header() http.Header { return w.header }
func (w *trendCaptureWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *trendCaptureWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	return w.body.Write(b)
}

func (a *app) withTrendHotCache(kind string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseTrendQuery(r.URL.Query(), kind == "detail")
		if err != nil {
			next(w, r)
			return
		}
		if (a.trendBrowserConfig().Enabled || a.trendLocalConfig().Enabled) && !q.archived() && kind != "options" {
			next(w, r)
			return
		}
		cfg := RedisConfig{}.withDefaults()
		if config := a.config.Load(); config != nil {
			cfg = config.(Config).Redis.withDefaults()
		}
		cache := a.trendCacheOverride
		if cache == nil {
			cache = redisTrendCache{config: cfg}
		}
		now := time.Now()
		bucket, expires, err := trendCacheNode(now, cfg)
		if err != nil {
			log.Print("trend cache reset configuration invalid")
			w.Header().Set("X-Trend-Cache", "UNAVAILABLE")
			next(w, r)
			return
		}
		language := "zh"
		if strings.HasPrefix(strings.ToLower(r.Header.Get("Accept-Language")), "en") {
			language = "en"
		}
		cacheKind := kind
		if kind == "options" {
			cacheKind = "options-browser-v2"
			if a.trendLocalConfig().Enabled {
				cacheKind = "options-local-v2"
			}
		}
		key := trendHotKey(cacheKind, q, language, bucket, cfg.KeyPrefix)
		cacheState := "MISS"
		if r.URL.Query().Get("refresh") != "1" {
			data, readErr := cache.Get(r.Context(), key)
			if readErr != nil {
				cacheState = "UNAVAILABLE"
				log.Print("trend cache read unavailable")
			} else if len(data) > 0 {
				var entry trendCacheRecord
				if json.Unmarshal(data, &entry) == nil && entry.ExpiresAt.After(now) && json.Valid(entry.Body) {
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.Header().Set("Cache-Control", "private, no-store")
					w.Header().Set("X-Trend-Cache", "HIT")
					w.Header().Set("X-Trend-Cache-Expires-At", entry.ExpiresAt.Format(time.RFC3339))
					w.Write(entry.Body)
					return
				}
			}
		}
		recorder := &trendCaptureWriter{header: w.Header().Clone()}
		next(recorder, r)
		var payload struct {
			Code int             `json:"code"`
			Data json.RawMessage `json:"data"`
		}
		if recorder.status == 200 && recorder.body.Len() <= trendCacheMaxBytes && json.Unmarshal(recorder.body.Bytes(), &payload) == nil && payload.Code == 0 && len(payload.Data) > 0 && time.Now().Before(expires) {
			data, _ := json.Marshal(trendCacheRecord{ExpiresAt: expires, Body: recorder.body.Bytes()})
			if err := cache.Set(r.Context(), key, data, expires); err != nil {
				cacheState = "UNAVAILABLE"
				log.Print("trend cache write unavailable")
			}
		} else {
			cacheState = "BYPASS"
		}
		for name, values := range recorder.header {
			w.Header()[name] = values
		}
		w.Header().Set("X-Trend-Cache", cacheState)
		w.Header().Set("X-Trend-Cache-Expires-At", expires.Format(time.RFC3339))
		w.Header().Set("Cache-Control", "private, no-store")
		status := recorder.status
		if status == 0 {
			status = 200
		}
		w.WriteHeader(status)
		w.Write(recorder.body.Bytes())
	}
}
