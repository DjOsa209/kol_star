package main

import (
	"context"
	"net/http"
	"testing"
)

func TestIPResearchRejectsScreenshotNoise(t *testing.T) {
	transport := recommendationEvidenceTransportFunc(func(*http.Request) (*http.Response, error) {
		return recommendationEvidenceTestResponse(200, `{"results":[
		{"title":"巴西农业技术初创公司将养殖场管理数字化","url":"https://www.wipo.int/news/agriculture","content":"腾讯分享电子游戏与数字经济的未来。"},
		{"title":"WIPO Lex","url":"https://www.wipo.int/wipolex/","content":"Intellectual property laws and treaties"},
		{"title":"2026 年 9 大 ISP 代理：提供商、功能与选择技巧","url":"https://proxy.example.com/isp","content":"电竞网络代理 IP 地址推荐"}
		]}`), nil
	})
	items, err := searchExternalIPResourcesWithTransport(context.Background(), "电竞领域有什么比较好的IP可以推荐给我", ipSearchProvider{name: "tavily", endpoint: "https://api.tavily.com/search"}, transport)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("irrelevant pages reached esports research: %d", len(items))
	}
}

func TestIPResearchWIPONewsIsNotBrandDatabase(t *testing.T) {
	item := labelIPResearchEvidence(recommendationEvidence{URL: "https://www.wipo.int/news/agriculture"})
	if item.SourceName == "WIPO Global Brand Database" || item.SourceCategory == "商标核查线索" {
		t.Fatalf("whole-site attribution mislabels an article: %+v", item)
	}
}
