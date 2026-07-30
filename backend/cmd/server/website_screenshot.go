package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

const websiteScreenshotTimeout = 25 * time.Second
const websiteScreenshotSettleDelay = 2 * time.Second

const websiteScreenshotOverlayCleanupScript = `(() => {
  const lowConsentPattern = /^(close|dismiss|reject( all)?|decline|only necessary|necessary only|关闭|仅必要|只允许必要|拒绝(全部|所有)?)$/i;
  const dismissPattern = /^(got it|close|dismiss|知道了|我知道了|关闭)$/i;
  const consentActionPattern = /^(accept( all| cookies)?|agree( to all)?|i agree|allow( all)?|接受(全部|所有)?|同意(全部|所有|并继续)?|我同意|我已阅读并同意|允许(全部|所有)?)$/i;
  const consentPattern = /(cookie|consent|privacy|gdpr|cmp|协议|隐私|同意)/i;
  const overlayPattern = /(cookie|consent|privacy|gdpr|cmp|modal|dialog|overlay|popup|pop-up|backdrop|lightbox|mask|协议|隐私|同意|弹窗|遮罩|提示|公告)/i;
  const roots = [];
  const documents = [];

  const collectRoots = root => {
    if (!root || roots.includes(root)) return;
    roots.push(root);
    for (const element of root.querySelectorAll('*')) {
      if (element.shadowRoot) collectRoots(element.shadowRoot);
      if (element.tagName === 'IFRAME') {
        try {
          if (element.contentDocument) collectDocument(element.contentDocument);
        } catch (_) {}
      }
    }
  };
  const collectDocument = currentDocument => {
    if (!currentDocument || documents.includes(currentDocument)) return;
    documents.push(currentDocument);
    collectRoots(currentDocument);
  };
  collectDocument(document);

  const visible = element => {
    const style = element.ownerDocument.defaultView.getComputedStyle(element);
    const rect = element.getBoundingClientRect();
    return style.display !== 'none' && style.visibility !== 'hidden' &&
      Number(style.opacity || 1) > 0 && rect.width > 1 && rect.height > 1;
  };
  const elementLabel = element => [
    element.innerText,
    element.value,
    element.getAttribute('aria-label'),
    element.getAttribute('title')
  ].filter(Boolean).join(' ').replace(/\s+/g, ' ').trim();
  const metadata = element => [
    element.id,
    typeof element.className === 'string' ? element.className : '',
    element.getAttribute('role'),
    element.getAttribute('aria-label'),
    element.getAttribute('title'),
    element.getAttribute('name')
  ].filter(Boolean).join(' ');
  const overlayContainer = element => {
    for (let current = element; current; current = current.parentElement) {
      if (current.matches('dialog,[role="dialog"],[aria-modal="true"]') ||
          overlayPattern.test(metadata(current))) return current;
    }
    return null;
  };
  const hasConsentContext = element => {
    for (let current = element; current; current = current.parentElement) {
      if (consentPattern.test(metadata(current))) return true;
    }
    return false;
  };

  let clicked = 0;
  const handledOverlays = new Set();
  for (const root of roots) {
    const controls = Array.from(root.querySelectorAll(
      'button, [role="button"], input[type="button"], input[type="submit"], a'
    )).filter(control => {
      if (!visible(control) || !overlayContainer(control)) return false;
      const label = elementLabel(control);
      return lowConsentPattern.test(label) || dismissPattern.test(label) ||
        (consentActionPattern.test(label) && hasConsentContext(control));
    });
    controls.sort((left, right) =>
      Number(!lowConsentPattern.test(elementLabel(left))) -
      Number(!lowConsentPattern.test(elementLabel(right)))
    );
    for (const control of controls) {
      const container = overlayContainer(control);
      if (!container || handledOverlays.has(container)) continue;
      try {
        control.click();
        clicked++;
        handledOverlays.add(container);
      } catch (_) {}
    }
  }

  let removed = 0;
  for (const root of roots) {
    for (const element of Array.from(root.querySelectorAll('*')).reverse()) {
      if (!element.isConnected || !visible(element)) continue;
      const view = element.ownerDocument.defaultView;
      const style = view.getComputedStyle(element);
      const rect = element.getBoundingClientRect();
      const viewportArea = Math.max(1, view.innerWidth * view.innerHeight);
      const visibleWidth = Math.max(0, Math.min(rect.right, view.innerWidth) - Math.max(rect.left, 0));
      const visibleHeight = Math.max(0, Math.min(rect.bottom, view.innerHeight) - Math.max(rect.top, 0));
      const coverage = visibleWidth * visibleHeight / viewportArea;
      const zIndex = Number.parseInt(style.zIndex, 10) || 0;
      const semanticDialog = element.matches('dialog,[role="dialog"],[aria-modal="true"]');
      const overlaySignal = overlayPattern.test(metadata(element));
      const positionedOverlay = ['fixed', 'sticky', 'absolute'].includes(style.position);
      const blocksViewport =
        (semanticDialog && coverage >= 0.08) ||
        (overlaySignal && coverage >= 0.08 && (positionedOverlay || zIndex >= 100)) ||
        (element.tagName === 'IFRAME' && overlaySignal && coverage >= 0.2);
      if (!blocksViewport) continue;
      element.remove();
      removed++;
    }
  }

  if (clicked || removed) {
    for (const currentDocument of documents) {
      for (const element of [currentDocument.documentElement, currentDocument.body]) {
        if (!element) continue;
        element.style.setProperty('overflow', 'auto', 'important');
        element.style.setProperty('pointer-events', 'auto', 'important');
      }
    }
  }
  return { clicked, removed };
})()`

func captureWebsiteScreenshot(
	ctx context.Context,
	resourceID int,
	cooperationID int,
	pageURL string,
) (string, string) {
	if err := validatePublicWebsiteURL(ctx, pageURL); err != nil {
		return "", "网页缩略图抓取失败：" + err.Error()
	}
	browserPath := websiteScreenshotBrowserPath()
	if browserPath == "" {
		return "", "网页缩略图抓取失败：服务器未安装 Chrome 或 Chromium"
	}

	relativeDir := filepath.Join(
		fmt.Sprintf("%d", resourceID),
		"project-content",
		fmt.Sprintf("%d", cooperationID),
	)
	targetDir := filepath.Join(resourceImageRoot, relativeDir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", "网页缩略图保存失败：" + err.Error()
	}
	tempFile, err := os.CreateTemp(targetDir, ".website-screenshot-*.png")
	if err != nil {
		return "", "网页缩略图保存失败：" + err.Error()
	}
	tempPath := tempFile.Name()
	_ = tempFile.Close()
	_ = os.Remove(tempPath)
	defer os.Remove(tempPath)

	captureCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), websiteScreenshotTimeout)
	defer cancel()
	err = captureWebsiteScreenshotToFile(
		captureCtx,
		browserPath,
		pageURL,
		tempPath,
		os.Geteuid() == 0,
	)
	if captureCtx.Err() != nil {
		return "", "网页缩略图抓取超时"
	}
	if err != nil {
		message := strings.TrimSpace(err.Error())
		if len(message) > 240 {
			message = message[len(message)-240:]
		}
		return "", "网页缩略图抓取失败：" + message
	}
	info, err := os.Stat(tempPath)
	if err != nil || info.Size() == 0 {
		return "", "网页缩略图抓取失败：浏览器未生成截图"
	}

	targetPath := filepath.Join(targetDir, "website.png")
	if err := os.Rename(tempPath, targetPath); err != nil {
		return "", "网页缩略图保存失败：" + err.Error()
	}
	return "/api/uploads/resource-images/" +
		filepath.ToSlash(filepath.Join(relativeDir, "website.png")), ""
}

func captureWebsiteScreenshotToFile(
	ctx context.Context,
	browserPath string,
	pageURL string,
	screenshotPath string,
	runningAsRoot bool,
) error {
	options := append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(browserPath),
		chromedp.DisableGPU,
		chromedp.WindowSize(1280, 720),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("force-device-scale-factor", "1"),
		chromedp.Flag("run-all-compositor-stages-before-draw", true),
	)
	// Chromium refuses to start as root with its sandbox enabled. Production
	// containers commonly run the backend as root, so disable it only in that
	// environment and keep the stronger default for regular service accounts.
	if runningAsRoot {
		options = append(options, chromedp.NoSandbox)
	}

	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(ctx, options...)
	defer cancelAllocator()
	browserCtx, cancelBrowser := chromedp.NewContext(allocatorCtx)
	defer cancelBrowser()

	// Native alert/confirm dialogs also block capture. Reject them while the
	// DOM cleanup below handles cookie consent and modal overlays.
	chromedp.ListenTarget(browserCtx, func(event any) {
		if _, ok := event.(*page.EventJavascriptDialogOpening); !ok {
			return
		}
		go func() {
			_ = chromedp.Run(browserCtx, page.HandleJavaScriptDialog(false))
		}()
	})

	var screenshot []byte
	if err := chromedp.Run(
		browserCtx,
		chromedp.EmulateViewport(1280, 720),
		chromedp.Navigate(pageURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(websiteScreenshotSettleDelay),
		chromedp.Evaluate(
			websiteScreenshotOverlayCleanupScript,
			nil,
			chromedp.EvalIgnoreExceptions,
		),
		chromedp.Sleep(250*time.Millisecond),
		chromedp.Evaluate(
			websiteScreenshotOverlayCleanupScript,
			nil,
			chromedp.EvalIgnoreExceptions,
		),
		chromedp.CaptureScreenshot(&screenshot),
	); err != nil {
		return err
	}
	if len(screenshot) == 0 {
		return fmt.Errorf("浏览器未生成截图")
	}
	return os.WriteFile(screenshotPath, screenshot, 0o644)
}

func websiteScreenshotBrowserPath() string {
	if configured := strings.TrimSpace(os.Getenv("KOL_CHROME_BIN")); configured != "" {
		if info, err := os.Stat(configured); err == nil && !info.IsDir() {
			return configured
		}
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	if runtime.GOOS == "darwin" {
		for _, path := range []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		} {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path
			}
		}
	}
	return ""
}
