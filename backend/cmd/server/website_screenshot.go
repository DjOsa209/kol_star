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
)

const websiteScreenshotTimeout = 25 * time.Second

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
	command := exec.CommandContext(
		captureCtx,
		browserPath,
		"--headless=new",
		"--disable-gpu",
		"--hide-scrollbars",
		"--window-size=1280,720",
		"--force-device-scale-factor=1",
		"--run-all-compositor-stages-before-draw",
		"--virtual-time-budget=4000",
		"--screenshot="+tempPath,
		pageURL,
	)
	output, err := command.CombinedOutput()
	if captureCtx.Err() != nil {
		return "", "网页缩略图抓取超时"
	}
	if err != nil {
		message := strings.TrimSpace(string(output))
		if len(message) > 240 {
			message = message[len(message)-240:]
		}
		return "", "网页缩略图抓取失败：" + firstNonEmpty(message, err.Error())
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
