package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ipMarketingFunction struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Fields   []string `json:"fields"`
	Required []string `json:"required"`
}

var ipMarketingFunctions = []ipMarketingFunction{
	{"imc", "IMC", []string{"marketHeat", "fanAudience", "commercialValue", "marketingRisks", "recentTrend", "socialFollowers", "brandValue"}, []string{"marketHeat", "fanAudience", "commercialValue", "marketingRisks", "recentTrend", "socialFollowers"}},
	{"brand", "品牌运营", []string{"ugcForecast"}, []string{"ugcForecast"}},
	{"user", "用户运营", []string{"audienceMatch"}, []string{"audienceMatch"}},
	{"media", "媒介投放", []string{"cpmValue"}, nil},
	{"pr", "PR", []string{"prOpinion"}, []string{"prOpinion"}},
}

type ipMarketingFunctionState struct {
	Version     int64    `json:"version"`
	Comments    string   `json:"comments"`
	Channels    []string `json:"channels"`
	SubmittedAt string   `json:"submittedAt,omitempty"`
	SubmittedBy int      `json:"submittedBy,omitempty"`
	UpdatedAt   string   `json:"updatedAt"`
	UpdatedBy   int      `json:"updatedBy"`
}
type ipMarketingInput struct {
	RequestID   int64                        `json:"requestId"`
	FunctionKey string                       `json:"functionKey"`
	Version     int64                        `json:"version"`
	SaveDraft   bool                         `json:"saveDraft"`
	Comments    string                       `json:"comments"`
	Channels    []string                     `json:"channels"`
	Candidates  map[string]map[string]string `json:"candidates"`
	Summary     map[string]string            `json:"summary"`
}

func ipMarketingFunctionByKey(key string) (ipMarketingFunction, bool) {
	for _, f := range ipMarketingFunctions {
		if f.Key == key {
			return f, true
		}
	}
	return ipMarketingFunction{}, false
}
func ipMarketingState(profile map[string]any, key string) ipMarketingFunctionState {
	functions, _ := profile["functions"].(map[string]any)
	raw, _ := json.Marshal(functions[key])
	var state ipMarketingFunctionState
	_ = json.Unmarshal(raw, &state)
	return state
}
func ipMarketingAllSubmitted(profile map[string]any) bool {
	for _, f := range ipMarketingFunctions {
		if ipMarketingState(profile, f.Key).SubmittedAt == "" {
			return false
		}
	}
	return true
}

// Only the chosen function's fields are patched into the freshly locked record.
// A whole browser snapshot must never replace colleagues' concurrent feedback.
func mergeIPMarketingOpinion(profile map[string]any, input ipMarketingInput, ids []int64, actor int, now time.Time) (map[string]any, string) {
	function, ok := ipMarketingFunctionByKey(input.FunctionKey)
	if !ok {
		return nil, "请选择本次填写的职能"
	}
	state := ipMarketingState(profile, input.FunctionKey)
	if state.SubmittedAt != "" {
		return nil, "该职能已提交，请刷新后查看"
	}
	if state.Version != input.Version {
		return nil, "该职能意见已被更新，请刷新后重新填写"
	}
	if !input.SaveDraft && strings.TrimSpace(input.Comments) == "" {
		return nil, "请填写本职能的补充意见"
	}
	if len(ids) == 0 {
		return nil, "请先完成意向IP评估"
	}
	allowed := map[string]bool{}
	for _, key := range function.Fields {
		allowed[key] = true
	}
	existingIDs := map[string]bool{}
	for _, id := range ids {
		existingIDs[strconv.FormatInt(id, 10)] = true
	}
	candidates, _ := profile["candidates"].(map[string]any)
	if candidates == nil {
		candidates = map[string]any{}
	}
	for id, entry := range input.Candidates {
		if !existingIDs[id] {
			return nil, "提交的IP不属于此需求，请刷新后重试"
		}
		stored, _ := candidates[id].(map[string]any)
		if stored == nil {
			stored = map[string]any{}
		}
		for key, value := range entry {
			if !allowed[key] {
				return nil, "只能提交所选职能的意见字段"
			}
			stored[key] = strings.TrimSpace(value)
		}
		candidates[id] = stored
	}
	if !input.SaveDraft {
		for _, id := range ids {
			entry, _ := candidates[strconv.FormatInt(id, 10)].(map[string]any)
			for _, key := range function.Required {
				if ipMapText(entry, key) == "" {
					return nil, "请补齐每个IP的" + function.Label + "意见，无需填写其他职能"
				}
			}
		}
	}
	functions, _ := profile["functions"].(map[string]any)
	if functions == nil {
		functions = map[string]any{}
	}
	state.Version++
	state.Comments = strings.TrimSpace(input.Comments)
	state.Channels = input.Channels
	state.UpdatedAt = now.In(ipTimeZone).Format(time.RFC3339)
	state.UpdatedBy = actor
	if !input.SaveDraft {
		state.SubmittedAt = state.UpdatedAt
		state.SubmittedBy = actor
	}
	// Keep the stored representation uniform for JSON reads and subsequent writes.
	raw, _ := json.Marshal(state)
	var saved map[string]any
	_ = json.Unmarshal(raw, &saved)
	functions[input.FunctionKey] = saved
	profile["functions"] = functions
	profile["candidates"] = candidates
	return profile, ""
}

func ipMarketingAggregates(profile map[string]any) (string, []string) {
	comments := []string{}
	channels := []string{}
	seen := map[string]bool{}
	if legacy := ipMapText(profile, "legacyMarketingComments"); legacy != "" {
		comments = append(comments, "历史补充意见："+legacy)
	}
	appendChannel := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			channels = append(channels, value)
		}
	}
	if legacy, ok := profile["legacyMarketingChannels"].([]any); ok {
		for _, v := range legacy {
			if s, ok := v.(string); ok {
				appendChannel(s)
			}
		}
	}
	for _, function := range ipMarketingFunctions {
		state := ipMarketingState(profile, function.Key)
		if state.Comments != "" {
			comments = append(comments, function.Label+"："+state.Comments)
		}
		for _, channel := range state.Channels {
			appendChannel(channel)
		}
	}
	return strings.Join(comments, "\n"), channels
}

func (a *app) ipRequestMarketing(w http.ResponseWriter, r *http.Request) {
	var input ipMarketingInput
	if !readIPJSON(w, r, &input) {
		return
	}
	if input.RequestID <= 0 {
		writeError(w, http.StatusBadRequest, 400, "缺少需求编号")
		return
	}
	if _, ok := ipMarketingFunctionByKey(input.FunctionKey); !ok {
		writeError(w, http.StatusBadRequest, 400, "请选择本次填写的职能")
		return
	}
	actor, ok := a.currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, 401, "未登录或登录已过期")
		return
	}
	tx, err := a.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer tx.Rollback()
	var status, raw, oldComments, oldChannels string
	summary := make([]string, 4)
	err = tx.QueryRowContext(r.Context(), `select status,coalesce(marketing_profile,''),coalesce(marketing_comments,''),coalesce(marketing_channels,''),market_heat,coalesce(fan_audience,''),commercial_value,coalesce(marketing_risks,'') from biz_ip_requests where id=? for update`, input.RequestID).Scan(&status, &raw, &oldComments, &oldChannels, &summary[0], &summary[1], &summary[2], &summary[3])
	if err != nil {
		writeDBError(w, err)
		return
	}
	if status != "ip_reviewed" {
		writeError(w, http.StatusConflict, 409, "需求未处于待营销补充状态")
		return
	}
	profile := map[string]any{}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &profile); err != nil {
			writeDBError(w, err)
			return
		}
	}
	if profile == nil {
		profile = map[string]any{}
	}
	state := ipMarketingState(profile, input.FunctionKey)
	if state.SubmittedAt != "" || state.Version != input.Version {
		writeError(w, http.StatusConflict, 409, "该职能意见已提交或被更新，请刷新后重试")
		return
	}
	// Retain old shared opinions as historical context rather than assigning them
	// to a function or accidentally counting them as five separate submissions.
	if _, exists := profile["functions"]; !exists {
		if oldComments != "" {
			profile["legacyMarketingComments"] = oldComments
		}
		var legacy []any
		if json.Unmarshal([]byte(oldChannels), &legacy) == nil {
			profile["legacyMarketingChannels"] = legacy
		}
	}
	rows, err := tx.QueryContext(r.Context(), "select ip_id from biz_ip_request_candidates where request_id=?", input.RequestID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		writeDBError(w, err)
		return
	}
	now := time.Now().In(ipTimeZone)
	profile, message := mergeIPMarketingOpinion(profile, input, ids, actor, now)
	if message != "" {
		writeError(w, http.StatusBadRequest, 400, message)
		return
	}
	if input.FunctionKey == "imc" {
		for index, key := range []string{"marketHeat", "fanAudience", "commercialValue", "marketingRisks"} {
			if value, exists := input.Summary[key]; exists {
				summary[index] = strings.TrimSpace(value)
			}
		}
	}
	complete := ipMarketingAllSubmitted(profile)
	var completedAt any
	if complete {
		status = "marketing_reviewed"
		completedAt = now.Format("2006-01-02 15:04:05")
	}
	comments, channels := ipMarketingAggregates(profile)
	_, err = tx.ExecContext(r.Context(), `update biz_ip_requests set market_heat=?,fan_audience=?,commercial_value=?,marketing_risks=?,marketing_channels=?,marketing_comments=?,marketing_profile=?,status=?,marketing_reviewed_at=? where id=? and status='ip_reviewed'`, summary[0], summary[1], summary[2], summary[3], ipJSON(channels), comments, ipJSON(profile), status, completedAt, input.RequestID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		writeDBError(w, err)
		return
	}
	if complete {
		a.notifyIPStageAsync(input.RequestID, "marketing_reviewed")
	}
	a.queueIPContent(r.Context(), "ip_request", input.RequestID)
	writeOK(w, map[string]any{"status": status, "functionKey": input.FunctionKey, "version": ipMarketingState(profile, input.FunctionKey).Version})
}
