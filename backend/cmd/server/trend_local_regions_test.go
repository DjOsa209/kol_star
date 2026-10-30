package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func batchFixture(t *testing.T, a *app) (string, string, map[string]any) {
	t.Helper()
	r := requestForTrendOwner("/business/trends/local-jobs?tab=tag&region=US&period=30&regions=US,GB,JP", 1)
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	a.trendLocalCreate(w, r)
	var response struct {
		Data map[string]any `json:"data"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil {
		t.Fatal("create batch failed")
	}
	return response.Data["id"].(string), response.Data["token"].(string), response.Data
}

func batchUpload(a *app, id, token, region string, raw []byte, failed bool) *httptest.ResponseRecorder {
	params := url.Values{"job": {id}, "region": {region}}
	if failed {
		params.Set("failed", "1")
	}
	r := httptest.NewRequest("POST", "/business/trends/local-upload?"+params.Encode(), bytes.NewReader(raw))
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.trendLocalUpload(w, r)
	return w
}

func waitBatchRegion(t *testing.T, j *trendLocalJob, region string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		j.mu.Lock()
		rows := j.regionStatus()
		j.mu.Unlock()
		for _, row := range rows {
			if row["region"] == region && row["state"] == "complete" {
				return
			}
			if row["region"] == region && row["state"] == "failed" {
				t.Fatal("region processing failed")
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("region processing timed out")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestTrendLocalBatchIsolationAndPartialFailure(t *testing.T) {
	a := localFixtureApp(t)
	id, token, data := batchFixture(t, a)
	j := a.localJob(id)
	// Reproduce the downloaded version-2 file with server-issued expiry.
	rawTask, _ := json.Marshal(map[string]any{"version": 2, "server": "http://localhost:8848/api", "job": id, "token": token, "expiresAt": data["expiresAt"], "regions": []string{"US", "GB", "JP"}, "query": map[string]string{"tab": "tag", "region": "US", "period": "30"}})
	if _, err := readLocalTrendTask(bytes.NewReader(rawTask), time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(j.children) != 3 || time.Until(j.ExpiresAt) < 89*time.Minute {
		t.Fatal("invalid batch scope or lifetime")
	}
	fixture := localFixtureDataset()
	raw, _ := json.Marshal(fixture)
	for _, region := range []string{"FR", ""} {
		if w := batchUpload(a, id, token, region, raw, false); w.Code != 403 {
			t.Fatal("unbound region accepted")
		}
	}
	if w := batchUpload(a, id, strings.Repeat("0", 64), "US", raw, false); w.Code != 401 {
		t.Fatal("invalid grant accepted")
	}
	if w := batchUpload(a, id, token, "US", raw, false); w.Code != 200 {
		t.Fatal("US upload failed")
	}
	waitBatchRegion(t, j, "US")
	if w := batchUpload(a, id, token, "US", raw, false); w.Code != 200 {
		t.Fatal("idempotent regional retry failed")
	}
	if w := batchUpload(a, id, token, "US", []byte("{}"), true); w.Code != 409 {
		t.Fatal("completed region overwritten by failure")
	}
	if w := batchUpload(a, id, token, "GB", []byte("{}"), true); w.Code != 200 {
		t.Fatal("failure report rejected")
	}
	fixture.Rows[0]["hashtagName"] = "JP fixture"
	raw, _ = json.Marshal(fixture)
	if w := batchUpload(a, id, token, "JP", raw, false); w.Code != 200 {
		t.Fatal("later region rejected after failure")
	}
	waitBatchRegion(t, j, "JP")
	j.mu.Lock()
	status := j.status()
	j.mu.Unlock()
	if status["state"] != "partial" {
		t.Fatal("partial completion not reported")
	}
	cache, cfg := a.trendRedis()
	bucket, _, _ := trendCacheNode(time.Now(), cfg)
	for _, region := range []string{"US", "GB", "JP"} {
		q := j.query
		q.Region = region
		data, _ := cache.Get(context.Background(), trendDatasetKey(1, q, bucket, cfg.KeyPrefix))
		if region == "GB" && len(data) != 0 {
			t.Fatal("failed region published")
		}
		if region != "GB" && len(data) == 0 {
			t.Fatal("successful region missing")
		}
		if region == "US" && bytes.Contains(data, []byte("JP fixture")) {
			t.Fatal("regions mixed in cache")
		}
		if region == "JP" && !bytes.Contains(data, []byte("JP fixture")) {
			t.Fatal("wrong regional cache")
		}
	}
}

func TestTrendLocalBatchCancellationAndValidation(t *testing.T) {
	for _, tc := range []struct{ tab, regions string }{{"tag", "US,US"}, {"tag", strings.Join(trendTagRegions, ",") + ",US"}, {"tag", "US,"}, {"tag", "US,../"}, {"video", "US,GB"}} {
		a := localFixtureApp(t)
		r := requestForTrendOwner("/business/trends/local-jobs?tab="+tc.tab+"&regions="+url.QueryEscape(tc.regions), 1)
		r.RemoteAddr = "127.0.0.1:1234"
		w := httptest.NewRecorder()
		a.trendLocalCreate(w, r)
		if w.Code != 400 {
			t.Fatal("invalid batch accepted")
		}
	}
	a := localFixtureApp(t)
	id, token, _ := batchFixture(t, a)
	r := requestForTrendOwner("/business/trends/local-jobs/"+id+"/cancel", 2)
	r.SetPathValue("job", id)
	w := httptest.NewRecorder()
	a.trendLocalCancel(w, r)
	if w.Code != 404 {
		t.Fatal("cross-owner cancel allowed")
	}
	r = requestForTrendOwner("/business/trends/local-jobs/"+id+"/cancel", 1)
	r.SetPathValue("job", id)
	w = httptest.NewRecorder()
	a.trendLocalCancel(w, r)
	fixture := localFixtureDataset()
	raw, _ := json.Marshal(fixture)
	for _, region := range []string{"US", "GB", "JP"} {
		if w := batchUpload(a, id, token, region, raw, false); w.Code != 409 {
			t.Fatal("cancelled child accepted upload")
		}
	}
}

func TestTrendLocalAllSupportedRegions(t *testing.T) {
	for _, tc := range []struct {
		tab     string
		regions []string
	}{
		{"tag", trendTagRegions}, {"video", trendVideoRegions}, {"tag", trendTagRegions[:6]},
	} {
		t.Run(tc.tab+"-"+strings.Join(tc.regions, "-"), func(t *testing.T) {
			a := localFixtureApp(t)
			params := url.Values{"tab": {tc.tab}, "region": {tc.regions[0]}, "regions": {strings.Join(tc.regions, ",")}}
			r := requestForTrendOwner("/business/trends/local-jobs?"+params.Encode(), 1)
			r.RemoteAddr = "127.0.0.1:1234"
			w := httptest.NewRecorder()
			a.trendLocalCreate(w, r)
			var response struct {
				Data struct {
					ID        string    `json:"id"`
					Token     string    `json:"token"`
					ExpiresAt time.Time `json:"expiresAt"`
				} `json:"data"`
			}
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil {
				t.Fatal("supported regions rejected")
			}
			j := a.localJob(response.Data.ID)
			if len(j.children) != len(tc.regions) {
				t.Fatal("region scope truncated")
			}
			validity := time.Duration(len(tc.regions)) * 30 * time.Minute
			if remaining := time.Until(j.ExpiresAt); remaining > validity || remaining < validity-time.Minute {
				t.Fatal("incorrect batch expiry")
			}
			task := localTrendTask{Version: 2, Server: "https://admin.example.test/api", Job: response.Data.ID, Token: response.Data.Token, Regions: tc.regions, Query: map[string]string{"tab": tc.tab, "region": tc.regions[0]}, ExpiresAt: response.Data.ExpiresAt}
			raw, _ := json.Marshal(task)
			if _, err := readLocalTrendTask(bytes.NewReader(raw), time.Now()); err != nil {
				t.Fatal("client rejected full server grant:", err)
			}
			queries, err := localTrendTaskQueries(task)
			if err != nil || len(queries) != len(tc.regions) {
				t.Fatal("client scope truncated")
			}
			for i, q := range queries {
				if q.Region != tc.regions[i] {
					t.Fatal("region order changed")
				}
			}
			task.ExpiresAt = time.Now().Add(validity + time.Hour)
			raw, _ = json.Marshal(task)
			if _, err := readLocalTrendTask(bytes.NewReader(raw), time.Now()); err == nil {
				t.Fatal("unbounded expiry accepted")
			}
			last := tc.regions[len(tc.regions)-1]
			if w := batchUpload(a, j.ID, response.Data.Token, last, []byte("{}"), true); w.Code != 200 {
				t.Fatal("last region is not authorized")
			}
			task.Regions = append(append([]string{}, tc.regions...), tc.regions[0])
			task.ExpiresAt = response.Data.ExpiresAt
			raw, _ = json.Marshal(task)
			if _, err := readLocalTrendTask(bytes.NewReader(raw), time.Now()); err == nil {
				t.Fatal("duplicate scope accepted by client")
			}
		})
	}
}

func TestTrendLocalBatchClientContinuesAfterFailure(t *testing.T) {
	queries, err := localTrendRegionQueries(trendQuery{Tab: "tag", Region: "US"}, "US,GB,JP")
	if err != nil {
		t.Fatal(err)
	}
	var collected, uploaded, failures []string
	count := runLocalTrendRegions(context.Background(), queries, func(q trendQuery) (*trendDataset, error) {
		collected = append(collected, q.Region)
		if q.Region == "GB" {
			return nil, errors.New("fixture failure")
		}
		d := localFixtureDataset()
		return &d, nil
	}, func(q trendQuery, d *trendDataset) error { uploaded = append(uploaded, q.Region); return nil }, func(q trendQuery, err error) { failures = append(failures, q.Region) })
	if count != 1 || strings.Join(collected, ",") != "US,GB,JP" || strings.Join(uploaded, ",") != "US,JP" || strings.Join(failures, ",") != "GB" {
		t.Fatal("batch did not continue in order")
	}
}
