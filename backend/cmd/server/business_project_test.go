package main

import "testing"

func TestProjectTargetMarketValue(t *testing.T) {
	if got := projectTargetMarketValue([]any{"中国", "美国", "中国", ""}); got != "中国,美国" {
		t.Fatalf("array target market = %q", got)
	}
	if got := projectTargetMarketValue("德国，法国;日本"); got != "德国,法国,日本" {
		t.Fatalf("string target market = %q", got)
	}
}

func TestProjectStatusValue(t *testing.T) {
	tests := map[string]string{
		"":          "未开始",
		"需求创建":      "未开始",
		"Paused":    "未开始",
		"Active":    "进行中",
		"进行中":       "进行中",
		"Completed": "已结束",
		"已完成":       "已结束",
	}
	for input, want := range tests {
		if got := projectStatusValue(input); got != want {
			t.Fatalf("projectStatusValue(%q) = %q, want %q", input, got, want)
		}
	}
}
