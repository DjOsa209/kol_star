package main

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

func localTrendRegionQueries(q trendQuery, raw string) ([]trendQuery, error) {
	regions := []string{q.Region}
	if raw != "" {
		regions = strings.Split(raw, ",")
	}
	allowed := trendTagRegions
	if q.Tab == "video" {
		allowed = trendVideoRegions
	}
	if len(regions) > len(allowed) {
		return nil, errors.New("too many regions")
	}
	seen := map[string]bool{}
	queries := make([]trendQuery, 0, len(regions))
	for _, region := range regions {
		if seen[region] || !contains(allowed, region) {
			return nil, errors.New("invalid region")
		}
		seen[region] = true
		next := q
		next.Region = region
		queries = append(queries, next)
	}
	return queries, nil
}

// Caller holds the parent mutex. Child workers only lock their own job.
func (j *trendLocalJob) regionStatus() []map[string]any {
	if len(j.children) == 0 {
		return []map[string]any{{"region": j.query.Region, "state": j.State, "message": j.Message, "imagesSaved": j.ImagesSaved, "imagesFailed": j.ImagesFailed}}
	}
	result := make([]map[string]any, 0, len(j.children))
	complete, waiting, processing := 0, 0, 0
	j.ImagesSaved, j.ImagesFailed = 0, 0
	for _, child := range j.children {
		child.mu.Lock()
		if child.State == "waiting_upload" && !time.Now().Before(child.ExpiresAt) {
			child.State, child.Message = "failed", "授权已过期，请重新采集"
		}
		result = append(result, child.regionStatus()[0])
		switch child.State {
		case "complete":
			complete++
		case "waiting_upload":
			waiting++
		case "processing":
			processing++
		}
		j.ImagesSaved += child.ImagesSaved
		j.ImagesFailed += child.ImagesFailed
		child.mu.Unlock()
	}
	if j.State != "cancelled" {
		switch {
		case processing > 0:
			j.State = "processing"
		case waiting > 0:
			j.State = "waiting_upload"
		case complete == len(j.children):
			j.State = "complete"
		case complete > 0:
			j.State = "partial"
		default:
			j.State = "failed"
		}
		j.Message = fmt.Sprintf("已完成 %d / %d 个地区", complete, len(j.children))
	}
	return result
}

func (j *trendLocalJob) cancelRegions(message string) {
	if j.State != "complete" {
		j.State, j.Message = "cancelled", message
		if j.cancel != nil {
			j.cancel()
		}
	}
	for _, child := range j.children {
		child.mu.Lock()
		child.cancelRegions(message)
		child.mu.Unlock()
	}
}

func (j *trendLocalJob) expireRegions() {
	for _, child := range j.children {
		child.mu.Lock()
		if child.State == "waiting_upload" {
			child.State, child.Message = "failed", "授权已过期，请重新采集"
		}
		child.mu.Unlock()
	}
}
