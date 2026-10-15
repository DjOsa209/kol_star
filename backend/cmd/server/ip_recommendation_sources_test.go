package main

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
)

func TestIPResearchSourcesUseCanonicalDomainsAndRejectImpersonation(t *testing.T) {
	for _, test := range []struct{ url, priority, name string }{
		{"https://www.licensingexpo.com/exhibitor-list/", "P0", "Licensing Expo"},
		{"https://www.brandlicensingexpo.com/exhibitor-list/brand-directory/", "P0", "Licensing Expo"},
		{"https://www.licenseglobal.com/licensing-resources/rankings-lists", "P0", "License Global"},
		{"https://licensinginternational.org/get-survey/", "P0", "Licensing International"},
		{"https://www.brandlicensing.eu/", "P1", "Brand Licensing Europe"},
		{"https://branddb.wipo.int/", "P1", "WIPO Global Brand Database"},
		{"https://commercial.yougov.com/report.pdf", "P2", "YouGov BrandIndex"},
		{"https://50mm.sportspro.com/", "P2", "SportsPro"},
		{"https://licensingexpo.com.attacker.com/", "", ""},
		{"https://notlicenseglobal.com/", "", ""},
	} {
		item := labelIPResearchEvidence(recommendationEvidence{URL: test.url})
		if item.SourceName != test.name || item.SourcePriority != test.priority || item.EvidenceUse == "" {
			t.Fatalf("source mismatch: %+v", item)
		}
	}
	domains := preferredIPResearchDomains()
	for _, domain := range []string{"licensingexpo.com", "brandlicensingexpo.com", "licenseglobal.com", "licensinginternational.org", "brandlicensing.eu", "wipo.int", "kantar.com", "yougov.com", "sportspro.com", "parrotanalytics.com", "billboard.com", "luminatedata.com", "escharts.com"} {
		if !slices.Contains(domains, domain) {
			t.Fatalf("missing preferred domain %s", domain)
		}
	}
}

func TestIPResearchPriorityRetainsActualEvidenceAndSourceDates(t *testing.T) {
	transport := recommendationEvidenceTransportFunc(func(req *http.Request) (*http.Response, error) {
		return recommendationEvidenceTestResponse(200, `{"results":[
		{"title":"Unknown artist","url":"https://artist.com/licensing","content":"Relevant public cooperation information"},
		{"title":"2025 Brand ranking","url":"https://yougov.com/report-2025","content":"2025 brand awareness report","published_date":"2025-01-20"},
		{"title":"Trademark","url":"https://branddb.wipo.int/name","content":"Trademark owner and registration geography"},
		{"title":"Directory A","url":"https://brandlicensingexpo.com/brands/a","content":"Public owner directory listing"},
		{"title":"Directory B","url":"https://licenseglobal.com/licensors/b","content":"Public licensing ranking"},
		{"title":"Industry survey","url":"https://licensinginternational.org/get-survey/","content":"Industry survey preview"}
		]}`), nil
	})
	items, err := searchExternalIPResourcesWithTransport(context.Background(), "public licensing", ipSearchProvider{name: "tavily", endpoint: "https://api.tavily.com/search"}, transport)
	if err != nil || len(items) != 6 {
		t.Fatalf("unexpected sources %v %+v", err, items)
	}
	if items[0].SourcePriority != "P0" || items[1].Title != "Directory B" || items[2].SourceCategory != "行业研究" || items[3].SourcePriority != "P1" || items[4].PublishedAt != "2025-01-20" || items[5].SourcePriority != "" {
		t.Fatalf("priority or dates lost: %+v", items)
	}
	encoded, _ := json.Marshal(items)
	if strings.Contains(string(encoded), "kantar.com") {
		t.Fatal("an unreturned preferred site became evidence")
	}
	if !strings.Contains(items[2].EvidenceUse, "行业数据不能") || !strings.Contains(ipResearchSourcePrompt, "未列明来源") || !strings.Contains(ipResearchSourcePrompt, "旧榜单") {
		t.Fatal("evidence use or freshness boundaries missing")
	}
}
