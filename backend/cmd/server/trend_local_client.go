package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"
)

type localTrendTask struct {
	Version   int               `json:"version"`
	Server    string            `json:"server"`
	Job       string            `json:"job"`
	Token     string            `json:"token"`
	Query     map[string]string `json:"query"`
	Regions   []string          `json:"regions,omitempty"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

func validLocalServer(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return u.Scheme == "http" && (strings.EqualFold(u.Hostname(), "localhost") || ip != nil && ip.IsLoopback())
}

func readLocalTrendTask(r io.Reader, now time.Time) (localTrendTask, error) {
	var task localTrendTask
	raw, err := io.ReadAll(io.LimitReader(r, 32769))
	if err != nil || len(raw) > 32768 {
		return task, errors.New("任务文件无法读取或超过大小限制，请重新下载本次任务")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&task) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return task, errors.New("任务文件格式不正确，请使用后台下载的 JSON 文件，不要手动修改")
	}
	if task.Version != 1 && task.Version != 2 {
		return task, errors.New("任务文件版本不支持，请重新下载采集程序和本次任务")
	}
	if !validLocalServer(task.Server) {
		return task, errors.New("任务中的服务器地址不受支持：远程后台必须使用 HTTPS，本机可使用 localhost 或回环 IP")
	}
	secret, tokenErr := hex.DecodeString(task.Token)
	id, idErr := hex.DecodeString(task.Job)
	if tokenErr != nil || idErr != nil || len(secret) != 32 || len(id) != 16 {
		return task, errors.New("任务文件缺少有效授权，请从后台重新创建并下载任务")
	}
	if task.ExpiresAt.IsZero() {
		return task, errors.New("任务文件缺少有效期，请从后台重新创建并下载任务")
	}
	if !now.Before(task.ExpiresAt) {
		return task, errors.New("本次授权已过期，请在后台重新采集并下载新任务，再执行窗口中的新命令；若刚下载，请核对电脑时间")
	}
	maxValidity := time.Hour
	if task.Version == 2 {
		if len(task.Regions) == 0 || len(task.Regions) > localTrendMaxRegions {
			return task, errors.New("任务地区列表不正确，请重新下载任务")
		}
		maxValidity = 3 * time.Hour
	} else if len(task.Regions) > 0 {
		return task, errors.New("任务版本与地区列表不匹配")
	}
	if task.ExpiresAt.After(now.Add(maxValidity)) {
		return task, errors.New("任务有效期与本机时间不一致，请核对电脑和服务器时间后重新下载任务")
	}
	return task, nil
}

func runLocalTrendClient(args []string) error {
	flags := flag.NewFlagSet("local-trends", flag.ContinueOnError)
	file := flags.String("job", "", "后台下载的本次采集任务 JSON 文件")
	endpoint := flags.String("cdp", "ws://127.0.0.1:9222/devtools/browser", "本机 Chrome 调试端点")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return errors.New("命令参数不正确")
	}
	if *file == "" || flags.NArg() != 0 || !validTrendAttachEndpoint(*endpoint) {
		return errors.New("用法: kol-trend-collector local-trends --job trend-job.json")
	}
	f, err := os.Open(*file)
	if err != nil {
		return errors.New("无法读取任务文件")
	}
	defer f.Close()
	task, err := readLocalTrendTask(f, time.Now())
	if err != nil {
		return err
	}
	values := url.Values{}
	for _, key := range []string{"tab", "region", "period", "orderByMetric"} {
		values.Set(key, task.Query[key])
	}
	q, err := parseTrendQuery(values, false)
	if err != nil {
		return errors.New("任务筛选条件不正确")
	}
	q.Live = true
	queries, err := localTrendRegionQueries(q, strings.Join(task.Regions, ","))
	if err != nil {
		return errors.New("任务地区列表不正确")
	}
	regions := make([]string, 0, len(queries))
	for _, query := range queries {
		regions = append(regions, query.Region)
	}
	input := bufio.NewReader(os.Stdin)
	fmt.Printf("本次采集: %s / %s / %d 天\n上传目的地: %s\n只上传榜单、详情和图片链接，不上传浏览器登录信息。\n确认目的地后输入 yes: ", q.Tab, strings.Join(regions, ", "), q.Period, task.Server)
	answer, _ := input.ReadString('\n')
	if strings.TrimSpace(answer) != "yes" {
		return errors.New("已取消")
	}
	fmt.Println("请先在普通 Chrome 中登录 TikTok Ads，并在 chrome://inspect/#remote-debugging 启用调试。准备好后按回车；Chrome 弹出连接确认时请允许。")
	if _, err = input.ReadString('\n'); err != nil {
		return errors.New("等待操作已取消")
	}
	base, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithDeadline(base, task.ExpiresAt)
	defer cancel()
	cfg := TrendBrowserConfig{Mode: "attach", Endpoint: *endpoint, AttachedUserID: 1}
	var driver trendBrowserDriver
	defer func() {
		if driver != nil {
			driver.Close()
		}
	}()
	collect := func(q trendQuery) (*trendDataset, error) {
		for {
			if driver == nil {
				driver, err = attachTrendChrome(ctx, cfg, 1)
				if err != nil {
					return nil, errors.New("无法连接本机 Chrome，请启用调试并允许连接后重试")
				}
				cfg.attachTarget = string(chromedpTargetID(driver.(*trendChrome)))
			}
			if err := driver.Open(ctx, q); err != nil {
				driver.Close()
				driver = nil
				return nil, errors.New("无法打开 Ads，请确认本机网络可访问")
			}
			requestCtx := context.WithValue(ctx, trendRequesterKey{}, trendRequester(driver.Request))
			data, err := collectTrendDataset(requestCtx, q, func(n int) {
				if n%10 == 0 {
					fmt.Printf("[%s] 已读取 %d / 100 条详情\n", q.Region, n)
				}
			})
			if errors.Is(err, errTrendLogin) {
				driver.Close()
				driver = nil
				fmt.Println("尚未取得完整榜单。已断开采集连接并保留页面，请在 Chrome 完成 Ads 登录后按回车继续。")
				if _, err = input.ReadString('\n'); err != nil {
					return nil, errors.New("等待登录已取消")
				}
				continue
			}
			if err != nil {
				driver.Close()
				driver = nil
				return nil, errors.New("完整榜单或详情未采集成功，未上传数据")
			}
			return data, nil
		}
	}
	failed := runLocalTrendRegions(ctx, queries, collect, func(q trendQuery, data *trendDataset) error {
		return uploadLocalTrendRegion(ctx, task, q.Region, data)
	}, func(q trendQuery, err error) {
		fmt.Printf("[%s] 未完成：%v；继续处理其他地区。\n", q.Region, err)
		_, _ = callLocalTrendUpload(ctx, task, q.Region, http.MethodPost, []byte("{}"), true)
	})
	if failed > 0 {
		return fmt.Errorf("批量采集结束：%d 个地区未完成，已成功地区的数据保留；可在后台单独重试失败地区", failed)
	}
	fmt.Printf("全部完成：%d 个地区的榜单、详情和缓存已更新。\n", len(queries))
	return nil
}

func runLocalTrendRegions(ctx context.Context, queries []trendQuery, collect func(trendQuery) (*trendDataset, error), upload func(trendQuery, *trendDataset) error, failed func(trendQuery, error)) int {
	failures := 0
	for _, q := range queries {
		var data *trendDataset
		err := ctx.Err()
		if err == nil {
			data, err = collect(q)
		}
		if err == nil {
			err = upload(q, data)
		}
		if err != nil {
			failures++
			failed(q, err)
		}
	}
	return failures
}

func callLocalTrendUpload(ctx context.Context, task localTrendTask, region, method string, body []byte, failed bool) (map[string]any, error) {
	params := url.Values{"job": {task.Job}, "region": {region}}
	if failed {
		params.Set("failed", "1")
	}
	endpointURL := strings.TrimRight(task.Server, "/") + "/business/trends/local-upload?" + params.Encode()
	client := &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, method, endpointURL, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("上传地址无效")
	}
	req.Header.Set("Authorization", "Bearer "+task.Token)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, errors.New("无法连接上传服务器，请检查网络")
	}
	defer response.Body.Close()
	var result struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if response.StatusCode != 200 || json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&result) != nil || result.Code != 0 {
		return nil, errors.New("服务器拒绝本次上传或授权已过期，请在后台查看任务状态")
	}
	return result.Data, nil
}

func uploadLocalTrendRegion(ctx context.Context, task localTrendTask, region string, data *trendDataset) error {
	raw, err := json.Marshal(data)
	if err != nil || len(raw) > trendCacheMaxBytes {
		return errors.New("采集结果超过上传限制")
	}
	call := func(method string, body []byte) (map[string]any, error) {
		return callLocalTrendUpload(ctx, task, region, method, body, false)
	}
	if _, err = call(http.MethodPost, raw); err != nil {
		return err
	}
	fmt.Printf("[%s] 已上传完整数据，服务器正在保存图片并更新缓存。\n", region)
	for {
		select {
		case <-ctx.Done():
			return errors.New("等待服务器处理超时，请在后台查看任务结果")
		case <-time.After(3 * time.Second):
		}
		status, err := call(http.MethodGet, nil)
		if err != nil {
			return err
		}
		switch status["state"] {
		case "complete":
			fmt.Printf("[%s] 完成：100 条榜单及详情已更新。图片保存 %v，失败 %v。\n", region, status["imagesSaved"], status["imagesFailed"])
			return nil
		case "failed", "cancelled":
			return errors.New("服务器处理未完成，请查看后台提示；旧缓存未被部分结果覆盖")
		}
	}
}
