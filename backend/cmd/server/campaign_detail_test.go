package main

import "testing"

func TestBuildCampaignDetailStatsIncludesCooperationCost(t *testing.T) {
	stats := buildCampaignDetailStats(
		map[string]any{"budget": float64(10000)},
		[]map[string]any{
			{
				"resourceId":        1,
				"quoteAmount":       float64(2500),
				"views":             float64(500000),
				"engagementCount":   float64(20000),
				"commentsCount":     float64(3000),
				"status":            "已发布",
				"deliverableStatus": "已完成",
			},
			{
				"resourceId":        2,
				"quoteAmount":       float64(1250),
				"impressions":       float64(250000),
				"engagementCount":   float64(10000),
				"commentsCount":     float64(1000),
				"status":            "已发布",
				"deliverableStatus": "已完成",
			},
		},
		nil,
	)
	if got := floatFromAny(stats["totalCost"]); got != 3750 {
		t.Fatalf("project totalCost = %v, want 3750: %#v", got, stats)
	}
}
