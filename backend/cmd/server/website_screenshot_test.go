package main

import (
	"context"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWebsiteScreenshotOmitsOpaqueSourcepointConsentFrame(t *testing.T) {
	messageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<style>
  html, body { width: 100%; height: 100%; margin: 0; background: rgb(245, 245, 245); }
</style>
<h1>We value your privacy</h1>
<button>Options</button>
<button>Accept All</button>`))
	}))
	defer messageServer.Close()

	pageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<style>
  html, body, main { width: 100%; height: 100%; margin: 0; }
  main { background: rgb(20, 180, 80); }
  .sp_veil_48291 { position: fixed; inset: 0; z-index: 10000; background: rgb(90, 90, 90); }
  .sp_message_iframe_48291 {
    position: fixed; left: 25%; top: 15%; width: 50%; height: 70%; z-index: 10001; border: 0;
  }
</style>
<main></main>
<script>
  setTimeout(() => {
    const veil = document.createElement('div');
    veil.className = 'sp_veil_48291';
    document.body.appendChild(veil);
    const frame = document.createElement('iframe');
    frame.className = 'sp_message_iframe_48291';
    frame.src = ` + "`" + messageServer.URL + "`" + `;
    document.body.appendChild(frame);
  }, 2600);
</script>`))
	}))
	defer pageServer.Close()

	assertWebsiteScreenshotCenterIsGreen(t, pageServer.URL)
}

func TestWebsiteScreenshotUsesNoConsentActionWithoutSemanticMarkup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<style>
  html, body, main { width: 100%; height: 100%; margin: 0; }
  main { background: rgb(20, 180, 80); }
  .a8f3 { position: fixed; inset: 0; z-index: 10000; background: rgb(90, 90, 90); }
  .b7e2 {
    position: fixed; left: 30%; top: 15%; width: 40%; height: 70%;
    z-index: 10001; background: white;
  }
</style>
<main></main>
<div class="a8f3" data-consent-fixture></div>
<div class="b7e2" data-consent-fixture>
  <h1>Welcome</h1>
  <h2>This site asks for your consent to use your personal data</h2>
  <button onclick="document.querySelectorAll('[data-consent-fixture]').forEach(element => element.remove())">Do not consent</button>
  <button onclick="document.querySelector('main').style.background='rgb(30, 40, 220)'">Consent</button>
</div>`))
	}))
	defer server.Close()

	assertWebsiteScreenshotCenterIsGreen(t, server.URL)
}

func TestWebsiteScreenshotPreservesOrdinaryFixedPageContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<style>
  html, body { width: 100%; height: 100%; margin: 0; }
  .fixed-app-shell { position: fixed; inset: 0; z-index: 1000; background: rgb(30, 40, 220); }
</style>
<main class="fixed-app-shell">
  <h1>Privacy engineering dashboard</h1>
  <p>Review consent analytics and personal data policies.</p>
</main>`))
	}))
	defer server.Close()

	red, green, blue := websiteScreenshotCenterPixel(t, server.URL)
	if blue <= red || blue <= green {
		t.Fatalf(
			"screenshot center pixel is (%d, %d, %d), want ordinary fixed content to remain blue",
			red,
			green,
			blue,
		)
	}
}

func assertWebsiteScreenshotCenterIsGreen(t *testing.T, pageURL string) {
	t.Helper()
	red, green, blue := websiteScreenshotCenterPixel(t, pageURL)
	if green <= red || green <= blue {
		t.Fatalf(
			"screenshot center pixel is (%d, %d, %d), want green page content without the consent overlay",
			red,
			green,
			blue,
		)
	}
}

func websiteScreenshotCenterPixel(t *testing.T, pageURL string) (uint32, uint32, uint32) {
	t.Helper()
	browserPath := websiteScreenshotBrowserPath()
	if browserPath == "" {
		t.Skip("Chrome or Chromium is not installed")
	}
	screenshotPath := filepath.Join(t.TempDir(), "website.png")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := captureWebsiteScreenshotToFile(
		ctx,
		browserPath,
		pageURL,
		screenshotPath,
		os.Geteuid() == 0,
	); err != nil {
		t.Fatalf("capture website screenshot: %v", err)
	}

	file, err := os.Open(screenshotPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	image, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	center := image.At(image.Bounds().Dx()/2, image.Bounds().Dy()/2)
	red, green, blue, _ := center.RGBA()
	return red >> 8, green >> 8, blue >> 8
}
