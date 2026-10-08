package main

import "testing"

func TestIPWeightedScore(t *testing.T) {
	assessment := map[string]any{"scoreDimensions": map[string]any{
		"audienceMatch":        float64(80),
		"marketCoverage":       float64(90),
		"scheduleAvailability": float64(60),
		"licenseRisk":          float64(100),
	}}
	got, valid := ipWeightedScore(assessment)
	if !valid || got != 82 {
		t.Fatalf("weighted score = %v, valid = %v; want 82, true", got, valid)
	}
	delete(assessment["scoreDimensions"].(map[string]any), "licenseRisk")
	if _, valid := ipWeightedScore(assessment); valid {
		t.Fatal("missing dimension must be invalid")
	}
	assessment["scoreDimensions"].(map[string]any)["licenseRisk"] = float64(101)
	if _, valid := ipWeightedScore(assessment); valid {
		t.Fatal("score above 100 must be invalid")
	}
}
