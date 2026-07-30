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
