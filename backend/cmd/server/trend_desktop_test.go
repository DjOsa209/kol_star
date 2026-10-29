package main

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTrendDesktopHandoffBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, state string
		enabled     bool
		owner, want int
		closed      bool
	}{
		{"disabled", "needs_login", false, 1, 503, false},
		{"other-owner", "needs_login", true, 2, 403, false},
		{"anonymous", "needs_login", true, 0, 403, false},
		{"collecting", "collecting", true, 1, 409, false},
		{"starting", "starting", true, 1, 409, false},
		{"manual-login", "needs_login", true, 1, 200, true},
		{"connection-failed", "failed", true, 1, 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &app{}
			a.config.Store(Config{TrendBrowser: TrendBrowserConfig{Enabled: true, Mode: "attach", DesktopEnabled: tc.enabled, AttachedUserID: 1}})
			driver := &fixtureBrowser{}
			session := &trendBrowserSession{lastUsed: time.Now(), browser: driver, job: &trendJob{ID: "desktop-fixture", State: tc.state, LoginMode: "external"}}
			a.trendManager().sessions[1] = session
			r := requestForTrendOwner("/business/trends/collect/desktop-fixture/desktop", tc.owner)
			r.SetPathValue("job", "desktop-fixture")
			w := httptest.NewRecorder()
			a.trendDesktopOpen(w, r)
			if w.Code != tc.want || driver.closed != tc.closed {
				t.Fatalf("handoff status/closure mismatch: %d/%v", w.Code, driver.closed)
			}
			if tc.closed && (session.browser != nil || !strings.Contains(w.Body.String(), "/trend-desktop/vnc.html")) {
				t.Fatal("manual login remained attached or missing fixed desktop path")
			}
		})
	}
}

func TestTrendDesktopWrongJobDoesNotDetach(t *testing.T) {
	a := &app{}
	a.config.Store(Config{TrendBrowser: TrendBrowserConfig{Enabled: true, Mode: "attach", DesktopEnabled: true, AttachedUserID: 1}})
	driver := &fixtureBrowser{}
	a.trendManager().sessions[1] = &trendBrowserSession{lastUsed: time.Now(), browser: driver, job: &trendJob{ID: "actual", State: "failed"}}
	r := requestForTrendOwner("/business/trends/collect/other/desktop", 1)
	r.SetPathValue("job", "other")
	w := httptest.NewRecorder()
	a.trendDesktopOpen(w, r)
	if w.Code != 404 || driver.closed {
		t.Fatal("wrong job detached Chrome")
	}
}
