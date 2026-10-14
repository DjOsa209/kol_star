package main

import (
	"encoding/json"
	"strings"
)

type ipRecommendationBasis struct {
	Label string `json:"label"`
	Value string `json:"value"`
}
type ipRecommendationAssessment struct {
	Verdict     string                  `json:"verdict"`
	Fit         string                  `json:"fit"`
	Basis       []ipRecommendationBasis `json:"basis"`
	EntryPoints []string                `json:"entryPoints"`
	Pending     []string                `json:"pending"`
}

// Use a versioned envelope in the existing advice column. Public advice stays
// plain text; creator recommendations and legacy snapshots need no migration.
type ipRecommendationAdviceEnvelope struct {
	Version    int                         `json:"version"`
	Advice     string                      `json:"advice"`
	Assessment *ipRecommendationAssessment `json:"assessment"`
}

func defaultIPRecommendationAssessment(s recommendationSubscription, c recommendationCandidate) *ipRecommendationAssessment {
	market := "授权区域需与版权方核对"
	if len(s.Markets) > 0 {
		market = "库内覆盖" + strings.Join(s.Markets, "、") + "，具体授权地域仍需确认"
	}
	fit := c.Reason
	if fit == "" {
		fit = "符合当前基础筛选条件，合作适配需进一步评估"
	}
	return &ipRecommendationAssessment{
		Verdict: "可进一步评估「" + c.Name + "」与本次需求的合作适配", Fit: fit,
		Basis:       []ipRecommendationBasis{{"需求契合", fit}, {"资源类型", recommendationString(c.Resource["ipType"])}, {"区域条件", market}},
		EntryPoints: recommendationDefaultScenarios(s, c), Pending: []string{"授权地域与素材使用范围", "合作报价、档期与版权方资源"},
	}
}
func cleanIPRecommendationAssessment(value, fallback *ipRecommendationAssessment) *ipRecommendationAssessment {
	if value == nil {
		return fallback
	}
	text := func(value string, max int) string {
		value = limitRecommendationText(value, max)
		if recommendationUnsupportedClaim(value) {
			return ""
		}
		return value
	}
	result := *fallback
	if v := text(value.Verdict, 100); v != "" {
		result.Verdict = v
	}
	if v := text(value.Fit, 140); v != "" {
		result.Fit = v
	}
	basis := []ipRecommendationBasis{}
	for _, row := range value.Basis {
		label, detail := text(row.Label, 16), text(row.Value, 140)
		if label != "" && detail != "" {
			basis = append(basis, ipRecommendationBasis{label, detail})
		}
		if len(basis) == 4 {
			break
		}
	}
	if len(basis) > 0 {
		result.Basis = basis
	}
	cleanList := func(values []string, max int) []string {
		list := []string{}
		for _, v := range values {
			if v = text(v, 100); v != "" {
				list = append(list, v)
			}
			if len(list) == max {
				break
			}
		}
		return list
	}
	if v := cleanList(value.EntryPoints, 3); len(v) > 0 {
		result.EntryPoints = v
	}
	result.Pending = cleanList(value.Pending, 3)
	result.Pending = append(result.Pending, "授权范围、报价与档期须与版权方确认")
	return &result
}
func storedRecommendationAdvice(c recommendationCandidate) string {
	if c.Assessment == nil {
		return c.Advice
	}
	encoded, _ := json.Marshal(ipRecommendationAdviceEnvelope{1, c.Advice, c.Assessment})
	return string(encoded)
}
func decodedRecommendationAdvice(value string) (string, *ipRecommendationAssessment) {
	var envelope ipRecommendationAdviceEnvelope
	if json.Unmarshal([]byte(value), &envelope) == nil && envelope.Version == 1 {
		return envelope.Advice, envelope.Assessment
	}
	return value, nil
}
