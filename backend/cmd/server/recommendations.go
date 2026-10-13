package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// A saved search belongs to exactly one user and one resource domain. The
// domain check below is deliberately inside these handlers: a shared route
// must never let an assistant user read IP searches (or vice versa).
type recommendationSubscription struct {
	ID             int64    `json:"id"`
	Domain         string   `json:"domain"`
	Name           string   `json:"name"`
	Query          string   `json:"query"`
	Markets        []string `json:"markets"`
	Platforms      []string `json:"platforms"`
	ResourceTypes  []string `json:"resourceTypes"`
	IPTypes        []string `json:"ipTypes"`
	Audience       string   `json:"audience"`
	Goal           string   `json:"goal"`
	Brand          string   `json:"brand"`
	Product        string   `json:"product"`
	BudgetCurrency string   `json:"budgetCurrency"`
	BudgetMin      float64  `json:"budgetMin"`
	BudgetMax      float64  `json:"budgetMax"`
	Active         bool     `json:"active"`
	RequestID      int64    `json:"requestId,omitempty"`
	LastRunAt      string   `json:"lastRunAt,omitempty"`
	CreatedAt      string   `json:"createdAt,omitempty"`
}

type recommendationSaveInput struct {
	recommendationSubscription
	ActiveInput *bool `json:"active"`
}

type recommendationCandidate struct {
	ID             int64
	Name           string
	Score          int
	Reason         string
	Risks          string
	Source         string
	Resource       map[string]any
	Advice         string
	AdviceSource   string
	Scenarios      []string
	Evidence       []recommendationEvidence
	EvidenceStatus string
	ResourceHash   string
}

// External references are returned by the fixed, server-side search provider.
// Model output can select these references by index, but cannot supply URLs.
type recommendationEvidence struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Source      string `json:"source"`
	PublishedAt string `json:"publishedAt,omitempty"`
	Summary     string `json:"summary,omitempty"`
}

func readRecommendationJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		writeError(w, http.StatusBadRequest, 400, "请求内容格式不正确")
		return false
	}
	return true
}

func (a *app) recommendationAccess(w http.ResponseWriter, r *http.Request, domain string) (int, bool) {
	userID, ok := a.currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, 401, "未登录或登录已过期")
		return 0, false
	}
	path := ""
	switch domain {
	case "creator":
		path = "/business/assistant"
	case "ip":
		path = "/business/ip/recommendations"
	default:
		writeError(w, http.StatusBadRequest, 400, "无效的推荐模块")
		return 0, false
	}
	allowed, err := a.hasMenu(r.Context(), userID, path)
	if err != nil {
		writeDBError(w, err)
		return 0, false
	}
	if !allowed {
		writeError(w, http.StatusForbidden, 403, "无菜单权限")
		return 0, false
	}
	return userID, true
}

func recommendationFiltersJSON(s recommendationSubscription) string {
	data, _ := json.Marshal(s)
	return string(data)
}

func decodeRecommendationSubscription(id int64, domain, name, queryText, filters string, active int, requestID sql.NullInt64, lastRun sql.NullTime, created time.Time) recommendationSubscription {
	s := recommendationSubscription{}
	_ = json.Unmarshal([]byte(filters), &s)
	s.ID, s.Domain, s.Name, s.Query = id, domain, name, queryText
	s.Active = active == 1
	if requestID.Valid {
		s.RequestID = requestID.Int64
	}
	if lastRun.Valid {
		s.LastRunAt = lastRun.Time.Format("2006-01-02 15:04")
	}
	s.CreatedAt = created.Format("2006-01-02 15:04")
	if s.Markets == nil {
		s.Markets = []string{}
	}
	if s.Platforms == nil {
		s.Platforms = []string{}
	}
	if s.ResourceTypes == nil {
		s.ResourceTypes = []string{}
	}
	if s.IPTypes == nil {
		s.IPTypes = []string{}
	}
	return s
}

func scanRecommendationSubscription(row *sql.Row) (recommendationSubscription, error) {
	var id int64
	var domain, name, queryText, filters string
	var active int
	var requestID sql.NullInt64
	var lastRun sql.NullTime
	var created time.Time
	err := row.Scan(&id, &domain, &name, &queryText, &filters, &active, &requestID, &lastRun, &created)
	if err != nil {
		return recommendationSubscription{}, err
	}
	return decodeRecommendationSubscription(id, domain, name, queryText, filters, active, requestID, lastRun, created), nil
}

const recommendationSubscriptionColumns = `id, domain, name, query_text, filters, active, request_id, last_run_at, created_at`

func (a *app) recommendationSubscriptionsList(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Domain string `json:"domain"`
	}
	if !readRecommendationJSON(w, r, &input) {
		return
	}
	userID, ok := a.recommendationAccess(w, r, input.Domain)
	if !ok {
		return
	}
	rows, err := a.DB().QueryContext(r.Context(), `select `+recommendationSubscriptionColumns+` from biz_recommendation_subscriptions where user_id=? and domain=? order by updated_at desc, id desc limit 100`, userID, input.Domain)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer rows.Close()
	list := make([]recommendationSubscription, 0)
	for rows.Next() {
		var id int64
		var domain, name, queryText, filters string
		var active int
		var requestID sql.NullInt64
		var lastRun sql.NullTime
		var created time.Time
		if err := rows.Scan(&id, &domain, &name, &queryText, &filters, &active, &requestID, &lastRun, &created); err != nil {
			writeDBError(w, err)
			return
		}
		list = append(list, decodeRecommendationSubscription(id, domain, name, queryText, filters, active, requestID, lastRun, created))
	}
	if err := rows.Err(); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"list": list})
}

func cleanRecommendationStrings(values []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 80 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
		if len(out) == 12 {
			break
		}
	}
	return out
}

func (a *app) recommendationSubscriptionSave(w http.ResponseWriter, r *http.Request) {
	var input recommendationSaveInput
	if !readRecommendationJSON(w, r, &input) {
		return
	}
	s := input.recommendationSubscription
	userID, ok := a.recommendationAccess(w, r, s.Domain)
	if !ok {
		return
	}
	var existing recommendationSubscription
	if s.ID > 0 {
		var err error
		existing, err = scanRecommendationSubscription(a.DB().QueryRowContext(r.Context(), `select `+recommendationSubscriptionColumns+` from biz_recommendation_subscriptions where id=? and user_id=? and domain=?`, s.ID, userID, s.Domain))
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, 404, "寻找条件不存在")
			return
		}
		if err != nil {
			writeDBError(w, err)
			return
		}
		// A pause/resume request may contain only id, domain, and active.
		if s.Query == "" && s.Name == "" && s.Markets == nil && s.Platforms == nil && s.ResourceTypes == nil && s.IPTypes == nil && s.Audience == "" && s.Goal == "" && s.BudgetMin == 0 && s.BudgetMax == 0 {
			s = existing
		}
	}
	s.Query = strings.TrimSpace(s.Query)
	s.Name = strings.TrimSpace(s.Name)
	s.Markets = cleanRecommendationStrings(s.Markets)
	s.Platforms = cleanRecommendationStrings(s.Platforms)
	s.ResourceTypes = cleanRecommendationStrings(s.ResourceTypes)
	s.IPTypes = cleanRecommendationStrings(s.IPTypes)
	if len(s.Query) > 1000 || len(s.Name) > 128 || len(s.Audience) > 255 || len(s.Goal) > 255 || s.BudgetMin < 0 || s.BudgetMax < 0 || (s.BudgetMax > 0 && s.BudgetMax < s.BudgetMin) {
		writeError(w, http.StatusBadRequest, 400, "寻找条件不合法")
		return
	}
	if s.Query == "" && len(s.Markets) == 0 && len(s.Platforms) == 0 && len(s.ResourceTypes) == 0 && len(s.IPTypes) == 0 && s.Audience == "" && s.Goal == "" {
		writeError(w, http.StatusBadRequest, 400, "请至少填写一项寻找条件")
		return
	}
	if s.Name == "" {
		s.Name = s.Query
		if len([]rune(s.Name)) > 30 {
			s.Name = string([]rune(s.Name)[:30])
		}
		if s.Name == "" {
			s.Name = "我的寻找条件"
		}
	}
	active := true
	if input.ActiveInput != nil {
		active = *input.ActiveInput
	}
	if s.ID > 0 {
		if input.ActiveInput == nil {
			active = existing.Active
		}
		tx, err := a.DB().BeginTx(r.Context(), nil)
		if err != nil {
			writeDBError(w, err)
			return
		}
		defer tx.Rollback()
		if recommendationCriteriaChanged(existing, s) {
			if _, err := tx.ExecContext(r.Context(), `delete n from biz_recommendation_notifications n join biz_recommendation_results r on r.id=n.result_id where r.subscription_id=? and n.user_id=?`, s.ID, userID); err != nil {
				writeDBError(w, err)
				return
			}
			if _, err := tx.ExecContext(r.Context(), `delete from biz_recommendation_results where subscription_id=?`, s.ID); err != nil {
				writeDBError(w, err)
				return
			}
			if _, err := tx.ExecContext(r.Context(), `delete from biz_recommendation_runs where subscription_id=?`, s.ID); err != nil {
				writeDBError(w, err)
				return
			}
		}
		_, err = tx.ExecContext(r.Context(), `update biz_recommendation_subscriptions set name=?, query_text=?, filters=?, active=?, request_id=? where id=? and user_id=? and domain=?`,
			s.Name, s.Query, recommendationFiltersJSON(s), active, nullableRecommendationID(s.RequestID), s.ID, userID, s.Domain)
		if err != nil {
			writeDBError(w, err)
			return
		}
		if err := tx.Commit(); err != nil {
			writeDBError(w, err)
			return
		}
	} else {
		result, err := a.DB().ExecContext(r.Context(), `insert into biz_recommendation_subscriptions (user_id,domain,name,query_text,filters,active,request_id) values (?,?,?,?,?,?,?)`,
			userID, s.Domain, s.Name, s.Query, recommendationFiltersJSON(s), active, nullableRecommendationID(s.RequestID))
		if err != nil {
			writeDBError(w, err)
			return
		}
		s.ID, err = result.LastInsertId()
		if err != nil {
			writeDBError(w, err)
			return
		}
	}
	// Saving a condition produces a first result set immediately. A failed
	// model call still yields rules. A database failure is visible and the saved
	// condition remains available for an explicit retry.
	if active {
		if _, err := a.executeRecommendationRun(r.Context(), s.ID, userID, s.Domain, fmt.Sprintf("save-%d", time.Now().UnixNano()), false); err != nil {
			log.Printf("initial recommendation run failed for subscription %d: %v", s.ID, err)
			warning := "寻找条件已保存，但推荐生成失败；请稍后点击立即推荐重试"
			writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: warning, Data: map[string]any{"id": s.ID, "warning": warning}})
			return
		}
	}
	writeOK(w, map[string]any{"id": s.ID})
}

func recommendationCriteriaChanged(previous, next recommendationSubscription) bool {
	return previous.Query != next.Query ||
		!slices.Equal(previous.Markets, next.Markets) ||
		!slices.Equal(previous.Platforms, next.Platforms) ||
		!slices.Equal(previous.ResourceTypes, next.ResourceTypes) ||
		!slices.Equal(previous.IPTypes, next.IPTypes) ||
		previous.Audience != next.Audience ||
		previous.Goal != next.Goal ||
		previous.Brand != next.Brand ||
		previous.Product != next.Product ||
		previous.BudgetCurrency != next.BudgetCurrency ||
		previous.BudgetMin != next.BudgetMin ||
		previous.BudgetMax != next.BudgetMax
}

func nullableRecommendationID(id int64) any {
	if id > 0 {
		return id
	}
	return nil
}

func (a *app) recommendationSubscriptionDelete(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID     int64  `json:"id"`
		Domain string `json:"domain"`
	}
	if !readRecommendationJSON(w, r, &input) {
		return
	}
	userID, ok := a.recommendationAccess(w, r, input.Domain)
	if !ok {
		return
	}
	if input.ID <= 0 {
		writeError(w, http.StatusBadRequest, 400, "缺少寻找条件编号")
		return
	}
	tx, err := a.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer tx.Rollback()
	var owned int64
	err = tx.QueryRowContext(r.Context(), `select id from biz_recommendation_subscriptions where id=? and user_id=? and domain=? for update`, input.ID, userID, input.Domain).Scan(&owned)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, 404, "寻找条件不存在")
		return
	}
	if err != nil {
		writeDBError(w, err)
		return
	}
	if _, err := tx.ExecContext(r.Context(), `delete n from biz_recommendation_notifications n join biz_recommendation_results r on r.id=n.result_id where r.subscription_id=? and n.user_id=?`, input.ID, userID); err != nil {
		writeDBError(w, err)
		return
	}
	if _, err := tx.ExecContext(r.Context(), `delete from biz_recommendation_results where subscription_id=?`, input.ID); err != nil {
		writeDBError(w, err)
		return
	}
	if _, err := tx.ExecContext(r.Context(), `delete from biz_recommendation_runs where subscription_id=?`, input.ID); err != nil {
		writeDBError(w, err)
		return
	}
	if _, err := tx.ExecContext(r.Context(), `delete from biz_recommendation_subscriptions where id=? and user_id=?`, input.ID, userID); err != nil {
		writeDBError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"id": input.ID})
}

func (a *app) recommendationRun(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Domain         string `json:"domain"`
		SubscriptionID int64  `json:"subscriptionId"`
	}
	if !readRecommendationJSON(w, r, &input) {
		return
	}
	userID, ok := a.recommendationAccess(w, r, input.Domain)
	if !ok {
		return
	}
	if input.SubscriptionID <= 0 {
		writeError(w, http.StatusBadRequest, 400, "缺少寻找条件编号")
		return
	}
	result, err := a.executeRecommendationRun(r.Context(), input.SubscriptionID, userID, input.Domain, fmt.Sprintf("manual-%d", time.Now().UnixNano()), false)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, 404, "寻找条件不存在")
		return
	}
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"results": result})
}

func recommendationMatches(haystack, needle string) bool {
	haystack, needle = strings.ToLower(strings.TrimSpace(haystack)), strings.ToLower(strings.TrimSpace(needle))
	return needle != "" && strings.Contains(haystack, needle)
}

func recommendationMatchesAny(haystack string, values []string) bool {
	for _, value := range values {
		if recommendationMatches(haystack, value) {
			return true
		}
	}
	return false
}

func recommendationMarketAliases(value string) []string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "美国", "us", "usa", "u.s.", "united states":
		return []string{"美国", "united states", "usa", "u.s.", "us"}
	case "英国", "uk", "united kingdom", "great britain":
		return []string{"英国", "united kingdom", "great britain", "uk"}
	case "德国", "germany":
		return []string{"德国", "germany"}
	case "日本", "japan":
		return []string{"日本", "japan"}
	case "加拿大", "canada":
		return []string{"加拿大", "canada"}
	default:
		return []string{value}
	}
}

func recommendationMarketMatchesAny(haystack string, values []string) bool {
	lower := strings.ToLower(strings.TrimSpace(haystack))
	if recommendationGlobalMarket(lower) {
		return true
	}
	for _, value := range values {
		if recommendationGlobalMarket(value) {
			return true
		}
		for _, alias := range recommendationMarketAliases(value) {
			alias = strings.ToLower(alias)
			if len(alias) <= 2 {
				if lower == alias || strings.Contains(lower, `"`+alias+`"`) {
					return true
				}
			} else if strings.Contains(lower, alias) {
				return true
			}
		}
	}
	return false
}

func recommendationGlobalMarket(value string) bool {
	return strings.Contains(strings.ToLower(value), "全球") || strings.Contains(strings.ToLower(value), "global")
}

// A global entry covers an individual market; a global search accepts all
// market entries. The same rule applies to creator and IP filters.
func recommendationIPMarketsMatch(markets any, requested []string) bool {
	if len(requested) == 0 {
		return true
	}
	return recommendationMarketMatchesAny(parseRecommendationMarkets(markets), requested)
}

func recommendationIPMarketSQLAliases(requested []string) []string {
	return recommendationAllMarketAliases(requested)
}

func recommendationCreatorActiveStatus(status string) bool {
	return strings.TrimSpace(status) == "可合作"
}

func recommendationIPActiveStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "可合作", "洽谈中", "待评估":
		return true
	default:
		return false
	}
}

// SQL filtering happens before LIMIT, so a large resource library cannot hide
// a matching market behind the first 300 unrelated rows. The in-memory check
// still applies the same aliases after the SQL prefilter.
func recommendationSQLAny(field string, values []string, jsonArray bool) (string, []any) {
	parts := make([]string, 0)
	args := make([]any, 0)
	seen := make(map[string]bool)
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		if len(value) <= 2 && !jsonArray {
			parts = append(parts, `lower(`+field+`)=?`)
			args = append(args, value)
		} else if len(value) <= 2 {
			parts = append(parts, `lower(`+field+`) like ?`)
			args = append(args, `%"`+value+`"%`)
		} else {
			parts = append(parts, `lower(`+field+`) like ?`)
			args = append(args, "%"+value+"%")
		}
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "(" + strings.Join(parts, " or ") + ")", args
}

func recommendationAllMarketAliases(markets []string) []string {
	values := make([]string, 0, len(markets)*3)
	for _, market := range markets {
		if recommendationGlobalMarket(market) {
			return nil
		}
		values = append(values, recommendationMarketAliases(market)...)
	}
	if len(values) > 0 {
		values = append(values, "全球", "global")
	}
	return values
}

func recommendationAllResourceTypeAliases(types []string) []string {
	values := make([]string, 0, len(types)*2)
	for _, resourceType := range types {
		values = append(values, resourceType)
		switch strings.ToLower(strings.TrimSpace(resourceType)) {
		case "创作者", "creator", "达人", "kol", "博主":
			values = append(values, "KOL", "创作者", "creator")
		case "媒体", "media":
			values = append(values, "媒体", "media")
		}
	}
	return values
}

func recommendationInferredIPTypes(query string) []string {
	for _, ipType := range []string{"动画", "动漫", "影视", "综艺", "游戏", "电竞", "体育", "潮牌", "潮玩", "艺术", "文化"} {
		if strings.Contains(query, ipType) {
			return []string{ipType}
		}
	}
	return nil
}

func recommendationIPTypeAliases(values []string) []string {
	aliases := make([]string, 0, len(values)*3)
	for _, value := range values {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "体育ip", "体育":
			aliases = append(aliases, "体育IP", "体育")
		case "动画/影视":
			aliases = append(aliases, "动画/影视", "动漫", "动画", "影视")
		case "动漫", "动画":
			aliases = append(aliases, "动画/影视", "动漫", "动画")
		case "影视":
			aliases = append(aliases, "动画/影视", "影视")
		case "线下活动", "音乐/活动", "音乐", "活动":
			aliases = append(aliases, "线下活动", "音乐/活动", "音乐", "活动")
		case "电竞", "电子竞技", "esports", "e-sports":
			aliases = append(aliases, "游戏", "电竞", "电子竞技", "esports", "e-sports")
		case "游戏":
			aliases = append(aliases, "游戏", "电竞", "电子竞技", "esports", "e-sports")
		case "文化艺术", "文化", "艺术":
			aliases = append(aliases, "文化艺术", "文化", "艺术")
		default:
			aliases = append(aliases, value)
		}
	}
	return aliases
}

func recommendationCreatorFilters(row map[string]any, markets, platforms, resourceTypes []string) bool {
	return (len(markets) == 0 || recommendationMarketMatchesAny(recommendationString(row["country"]), markets)) &&
		(len(platforms) == 0 || recommendationMatchesAny(recommendationString(row["platform"]), platforms)) &&
		(len(resourceTypes) == 0 || recommendationMatchesAny(recommendationString(row["resourceType"]), resourceTypes))
}

func parseRecommendationMarkets(value any) string {
	if markets, ok := value.([]string); ok {
		return strings.Join(markets, " ")
	}
	text := fmt.Sprint(value)
	var markets []string
	if json.Unmarshal([]byte(text), &markets) == nil {
		return strings.Join(markets, " ")
	}
	if text == "<nil>" {
		return ""
	}
	return text
}

func recommendationString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func (a *app) recommendationProductExpression(ctx context.Context) string {
	var count int
	err := a.DB().QueryRowContext(ctx, `select count(distinct table_name) from information_schema.tables where table_schema=database() and table_name in ('biz_cooperation_products','biz_discovery_products','biz_discovery_brands')`).Scan(&count)
	if err != nil || count != 3 {
		return `'' as confirmedProducts`
	}
	return `coalesce((select group_concat(distinct concat(b.name, ' ', p.name) separator '、') from biz_cooperation_products cp join biz_cooperations c on c.id=cp.cooperation_id join biz_discovery_products p on p.id=cp.product_id join biz_discovery_brands b on b.id=p.brand_id where c.resource_id=r.id and cp.status='confirmed' and c.status='已发布' and trim(coalesce(c.final_link,''))<>''), '') as confirmedProducts`
}

func (a *app) recommendationCandidates(ctx context.Context, s recommendationSubscription) ([]recommendationCandidate, error) {
	if s.Domain == "creator" {
		return a.creatorRecommendationCandidates(ctx, s)
	}
	return a.ipRecommendationCandidates(ctx, s)
}

type creatorRecommendationCriteria struct {
	parsed        map[string]any
	markets       []string
	platforms     []string
	resourceTypes []string
}

func deriveCreatorRecommendationCriteria(s recommendationSubscription) creatorRecommendationCriteria {
	parsed := parseAssistantDemand(map[string]any{"targetMarket": firstRecommendationString(s.Markets), "platform": firstRecommendationString(s.Platforms), "resourceType": firstRecommendationString(s.ResourceTypes), "budget": s.BudgetMax}, s.Query)
	markets := s.Markets
	if len(markets) == 0 && str(parsed, "targetMarket") != "" {
		markets = []string{str(parsed, "targetMarket")}
	}
	platforms := s.Platforms
	if len(platforms) == 0 && str(parsed, "platform") != "" {
		platforms = []string{str(parsed, "platform")}
	}
	resourceTypes := s.ResourceTypes
	if len(resourceTypes) == 0 && str(parsed, "resourceType") != "" {
		resourceTypes = []string{str(parsed, "resourceType")}
	}
	resourceTypes = recommendationAllResourceTypeAliases(resourceTypes)
	return creatorRecommendationCriteria{parsed: parsed, markets: markets, platforms: platforms, resourceTypes: resourceTypes}
}

func scoreCreatorRecommendation(row map[string]any, s recommendationSubscription, criteria creatorRecommendationCriteria) (recommendationCandidate, bool) {
	id := int64(intField(row, "id"))
	if id <= 0 || !recommendationCreatorActiveStatus(recommendationString(row["status"])) ||
		!recommendationCreatorFilters(row, criteria.markets, criteria.platforms, criteria.resourceTypes) {
		return recommendationCandidate{}, false
	}
	score, reasons, downgrades := recommendationScore(row, criteria.parsed)
	products := recommendationString(row["confirmedProducts"])
	if list, ok := row["confirmedProducts"].([]string); ok {
		products = strings.Join(list, "、")
	}
	if products != "" {
		score += 7
		reasons = append(reasons, "有已确认且已发布的产品合作")
		if recommendationMatches(products, s.Brand) || recommendationMatches(products, s.Product) || recommendationMatches(products, s.Query) {
			score += 13
			reasons = append(reasons, "历史合作产品匹配")
		}
	}
	if s.Audience != "" && recommendationMatches(recommendationString(row["category"])+" "+recommendationString(row["industry"]), s.Audience) {
		score += 10
		reasons = append(reasons, "受众方向匹配")
	}
	cost := estimateResourceCost(row)
	risks := append([]string{}, downgrades...)
	if s.BudgetMax > 0 {
		if s.BudgetCurrency == "" || strings.EqualFold(s.BudgetCurrency, "USD") {
			if float64(cost) > s.BudgetMax {
				return recommendationCandidate{}, false
			}
			reasons = append(reasons, "预估费用在预算内")
		} else {
			risks = append(risks, "预估费用币种为 USD，预算币种需核对")
		}
	}
	if len(reasons) == 0 {
		reasons = []string{"资源综合评分较高"}
	}
	row["confirmedProducts"] = products
	row["estimatedCost"] = cost
	row["estimatedCostCurrency"] = "USD"
	return recommendationCandidate{ID: id, Name: recommendationString(row["name"]), Score: clampInt(score, 0, 100), Reason: strings.Join(reasons, "；"), Risks: strings.Join(risks, "；"), Source: "rule", Resource: row}, true
}

func (a *app) creatorRecommendationCandidates(ctx context.Context, s recommendationSubscription) ([]recommendationCandidate, error) {
	productExpression := a.recommendationProductExpression(ctx)
	criteria := deriveCreatorRecommendationCriteria(s)
	where := ` where r.status = '可合作'`
	args := []any{}
	for _, filter := range []struct {
		field     string
		values    []string
		jsonArray bool
	}{
		{"r.country", recommendationAllMarketAliases(criteria.markets), false},
		{"r.platform", criteria.platforms, false},
		{"r.resource_type", criteria.resourceTypes, false},
	} {
		clause, values := recommendationSQLAny(filter.field, filter.values, filter.jsonArray)
		if clause != "" {
			where += " and " + clause
			args = append(args, values...)
		}
	}
	rows, err := a.queryMaps(ctx, `select r.id,r.name,r.avatar_url as avatarUrl,r.resource_type as resourceType,r.country,r.language,r.platform,r.industry,r.category,r.content_types as contentTypes,r.status,r.followers,r.engagement_rate as engagementRate,r.avg_views as avgViews,r.score,r.level,r.risk_level as riskLevel,`+productExpression+` from biz_resources r`+where+` order by r.score desc,r.updated_at desc limit 300`, args...)
	if err != nil {
		return nil, err
	}
	candidates := make([]recommendationCandidate, 0, len(rows))
	for _, row := range rows {
		if candidate, ok := scoreCreatorRecommendation(row, s, criteria); ok {
			candidates = append(candidates, candidate)
		}
	}
	sortRecommendationCandidates(candidates)
	if len(candidates) > 30 {
		candidates = candidates[:30]
	}
	return candidates, nil
}

func firstRecommendationString(values []string) string {
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func deriveIPRecommendationCriteria(s recommendationSubscription) recommendationSubscription {
	if len(s.Markets) == 0 {
		if inferred := detectMarket(s.Query); inferred != "" {
			s.Markets = []string{inferred}
		}
	}
	if len(s.IPTypes) == 0 {
		s.IPTypes = recommendationInferredIPTypes(s.Query)
	}
	return s
}

func (a *app) ipRecommendationCandidates(ctx context.Context, s recommendationSubscription) ([]recommendationCandidate, error) {
	s = deriveIPRecommendationCriteria(s)
	where := ` where r.cooperation_status in ('可合作','洽谈中','待评估')`
	args := []any{}
	for _, filter := range []struct {
		field     string
		values    []string
		jsonArray bool
	}{
		{"r.markets", recommendationIPMarketSQLAliases(s.Markets), true},
		{"r.ip_type", recommendationIPTypeAliases(s.IPTypes), false},
	} {
		clause, values := recommendationSQLAny(filter.field, filter.values, filter.jsonArray)
		if clause != "" {
			where += " and " + clause
			args = append(args, values...)
		}
	}
	rows, err := a.queryMaps(ctx, `select r.id,r.name,r.ip_type as ipType,r.markets,r.audience,r.summary,r.cooperation_status as cooperationStatus,r.currency,r.price_min as priceMin,r.price_max as priceMax,r.license_notes as licenseNotes,(select f.id from biz_ip_files f where f.ip_id=r.id and f.file_kind='visual' order by f.id desc limit 1) as visualFileId from biz_ip_resources r`+where+` order by r.updated_at desc,r.id desc limit 300`, args...)
	if err != nil {
		return nil, err
	}
	candidates := make([]recommendationCandidate, 0, len(rows))
	for _, row := range rows {
		if candidate, ok := scoreIPRecommendation(row, s); ok {
			candidates = append(candidates, candidate)
		}
	}
	sortRecommendationCandidates(candidates)
	if len(candidates) > 30 {
		candidates = candidates[:30]
	}
	return candidates, nil
}

func scoreIPRecommendation(row map[string]any, s recommendationSubscription) (recommendationCandidate, bool) {
	id := int64(intField(row, "id"))
	if id <= 0 {
		return recommendationCandidate{}, false
	}
	if !recommendationIPMarketsMatch(row["markets"], s.Markets) {
		return recommendationCandidate{}, false
	}
	if len(s.IPTypes) > 0 && !recommendationMatchesAny(recommendationString(row["ipType"]), recommendationIPTypeAliases(s.IPTypes)) {
		return recommendationCandidate{}, false
	}
	status := recommendationString(row["cooperationStatus"])
	if !recommendationIPActiveStatus(status) {
		return recommendationCandidate{}, false
	}
	score := 35
	reasons := []string{}
	risks := []string{}
	if len(s.Markets) > 0 {
		score += 20
		reasons = append(reasons, "覆盖目标市场")
	}
	if len(s.IPTypes) > 0 {
		score += 18
		reasons = append(reasons, "IP 类型匹配")
	}
	if s.Audience != "" && recommendationMatches(recommendationString(row["audience"]), s.Audience) {
		score += 12
		reasons = append(reasons, "目标受众匹配")
	}
	if s.Goal != "" && recommendationMatches(recommendationString(row["summary"])+" "+recommendationString(row["licenseNotes"]), s.Goal) {
		score += 9
		reasons = append(reasons, "合作目标与资料匹配")
	}
	if s.Query != "" && recommendationMatches(recommendationString(row["name"])+" "+recommendationString(row["summary"])+" "+recommendationString(row["audience"]), s.Query) {
		score += 10
		reasons = append(reasons, "关键词匹配")
	}
	if status == "可合作" {
		score += 10
		reasons = append(reasons, "当前可合作")
	} else {
		risks = append(risks, "合作状态需进一步确认")
	}
	priceMin := floatField(row, "priceMin")
	if s.BudgetMax > 0 {
		if s.BudgetCurrency != "" && !strings.EqualFold(s.BudgetCurrency, recommendationString(row["currency"])) {
			risks = append(risks, "参考价与预算币种不同，价格待确认")
		} else if priceMin > 0 && priceMin > s.BudgetMax {
			return recommendationCandidate{}, false
		} else if priceMin > 0 {
			score += 8
			reasons = append(reasons, "参考权益价格在预算内")
		} else {
			risks = append(risks, "缺少参考权益价格")
		}
	}
	if len(reasons) == 0 {
		reasons = []string{"符合基础筛选条件"}
	}
	marketList, _ := row["markets"].([]string)
	if marketList == nil {
		_ = json.Unmarshal([]byte(recommendationString(row["markets"])), &marketList)
	}
	if marketList == nil {
		marketList = []string{}
	}
	row["markets"] = marketList
	if row["priceMin"] != nil {
		row["priceMin"] = priceMin
	}
	if row["priceMax"] != nil {
		row["priceMax"] = floatField(row, "priceMax")
	}
	if row["visualFileId"] != nil {
		row["visualFileId"] = intField(row, "visualFileId")
	}
	return recommendationCandidate{ID: id, Name: recommendationString(row["name"]), Score: clampInt(score, 0, 100), Reason: strings.Join(reasons, "；"), Risks: strings.Join(risks, "；"), Source: "rule", Resource: row}, true
}

func sortRecommendationCandidates(values []recommendationCandidate) {
	sort.SliceStable(values, func(i, j int) bool {
		if values[i].Score == values[j].Score {
			return values[i].ID < values[j].ID
		}
		return values[i].Score > values[j].Score
	})
}

type recommendationAIRank struct {
	ID              int64    `json:"id"`
	Score           int      `json:"score"`
	Reason          string   `json:"reason"`
	Advice          string   `json:"advice"`
	Scenarios       []string `json:"scenarios"`
	ExternalInsight string   `json:"externalInsight"`
	EvidenceIDs     []int    `json:"evidenceIds"`
}

func filterRecommendationAIRanks(candidates []recommendationCandidate, rankings []recommendationAIRank) []recommendationCandidate {
	allowed := make(map[int64]recommendationCandidate, len(candidates))
	for _, c := range candidates {
		allowed[c.ID] = c
	}
	out := make([]recommendationCandidate, 0, len(candidates))
	used := make(map[int64]bool)
	for _, rank := range rankings {
		c, ok := allowed[rank.ID]
		if !ok || used[rank.ID] {
			continue
		}
		used[rank.ID] = true
		if rank.Score >= 1 && rank.Score <= 100 {
			c.Score = rank.Score
		}
		if reason := strings.TrimSpace(rank.Reason); reason != "" {
			if len([]rune(reason)) > 160 {
				reason = string([]rune(reason)[:160])
			}
			c.Reason = reason
		}
		if advice := limitRecommendationText(rank.Advice, 420); recommendationValidAIAdvice(advice, c) {
			c.Advice = advice
			if caution := recommendationRightsCaution(c); caution != "" {
				c.Advice += " " + caution
			}
			c.AdviceSource = "ai"
		}
		if c.AdviceSource == "ai" {
			scenarios := make([]string, 0, 3)
			for _, scenario := range rank.Scenarios {
				if scenario = limitRecommendationText(scenario, 100); scenario != "" && !recommendationUnsupportedClaim(scenario) {
					scenarios = append(scenarios, scenario)
				}
				if len(scenarios) == 3 {
					break
				}
			}
			if len(scenarios) > 0 {
				c.Scenarios = scenarios
			}
			selected := selectedRecommendationEvidence(c.Evidence, rank.EvidenceIDs)
			if insight := limitRecommendationText(rank.ExternalInsight, 260); insight != "" && len(selected) > 0 && !recommendationUnsupportedClaim(insight) {
				c.Advice += " 外部背景：" + insight
				c.Evidence = selected
			}
		}
		c.Source = "ai"
		out = append(out, c)
		if len(out) == 20 {
			break
		}
	}
	for _, c := range candidates {
		if len(out) == 20 {
			break
		}
		if !used[c.ID] {
			out = append(out, c)
		}
	}
	return out
}

func (a *app) requestRecommendationAIRanks(ctx context.Context, model assistantAIModel, s recommendationSubscription, candidates []recommendationCandidate) ([]recommendationAIRank, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	endpoint := model.BaseURL
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}
	safe := make([]map[string]any, 0, len(candidates))
	for _, c := range candidates {
		item := map[string]any{"id": c.ID, "name": c.Name, "ruleScore": c.Score, "ruleReason": c.Reason}
		if s.Domain == "creator" {
			for _, key := range []string{"resourceType", "country", "platform", "industry", "category", "confirmedProducts", "followers", "avgViews", "engagementRate", "estimatedCost", "estimatedCostCurrency", "riskLevel"} {
				item[key] = c.Resource[key]
			}
		} else {
			for _, key := range []string{"ipType", "markets", "audience", "summary", "cooperationStatus", "currency", "priceMin", "priceMax", "licenseNotes"} {
				item[key] = c.Resource[key]
			}
			publicEvidence := make([]map[string]any, 0, len(c.Evidence))
			for index, evidence := range c.Evidence {
				publicEvidence = append(publicEvidence, map[string]any{
					"id": index + 1, "title": evidence.Title, "source": evidence.Source,
					"summary": evidence.Summary,
				})
			}
			item["publicEvidence"] = publicEvidence
		}
		safe = append(safe, item)
		if len(safe) == 20 {
			break
		}
	}
	payload, _ := json.Marshal(map[string]any{"domain": s.Domain, "query": s.Query, "filters": s, "candidates": safe})
	body, _ := json.Marshal(map[string]any{
		"model": model.Model,
		"messages": []map[string]string{
			{"role": "system", "content": recommendationAdviceSystemPrompt()},
			{"role": "user", "content": string(payload)},
		},
		"temperature":     model.Temperature,
		"max_tokens":      5000,
		"response_format": map[string]string{"type": "json_object"},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+model.APIKey)
	resp, err := (&http.Client{Timeout: model.Timeout}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("模型服务状态 %d", resp.StatusCode)
	}
	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&completion); err != nil {
		return nil, err
	}
	if len(completion.Choices) == 0 {
		return nil, errors.New("模型未返回内容")
	}
	var parsed struct {
		Recommendations []recommendationAIRank `json:"recommendations"`
	}
	if err := json.Unmarshal([]byte(cleanJSONContent(completion.Choices[0].Message.Content)), &parsed); err != nil {
		return nil, err
	}
	return parsed.Recommendations, nil
}

func (a *app) executeRecommendationRun(ctx context.Context, subscriptionID int64, userID int, domain, runKey string, weekly bool) ([]map[string]any, error) {
	s, err := scanRecommendationSubscription(a.DB().QueryRowContext(ctx, `select `+recommendationSubscriptionColumns+` from biz_recommendation_subscriptions where id=? and user_id=? and domain=?`, subscriptionID, userID, domain))
	if err != nil {
		return nil, err
	}
	if weekly && !s.Active {
		return []map[string]any{}, nil
	}
	if weekly {
		var alreadyRun int
		if err := a.DB().QueryRowContext(ctx, `select count(*) from biz_recommendation_runs where subscription_id=? and run_key=?`, s.ID, runKey).Scan(&alreadyRun); err != nil {
			return nil, err
		}
		if alreadyRun > 0 {
			return []map[string]any{}, nil
		}
	}
	candidates, err := a.recommendationCandidates(ctx, s)
	if err != nil {
		return nil, err
	}
	prepareRecommendationAdvice(s, candidates)
	if s.Domain == "ip" && len(candidates) > 0 {
		// External context is independent of the model. Bound the public
		// lookup so rule advice can still show sources without delaying saves.
		evidenceCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		for i := 0; i < len(candidates) && i < 3 && evidenceCtx.Err() == nil; i++ {
			candidates[i].Evidence, candidates[i].EvidenceStatus = fetchIPRecommendationEvidence(evidenceCtx, candidates[i].Name, recommendationString(candidates[i].Resource["ipType"]))
		}
		cancel()
	}
	source := "rule"
	if model, configured := a.assistantAIModel(ctx); configured && model.EnableRecommendationReason && len(candidates) > 0 {
		// This run also writes results and reloads current resource facts.
		// Leave ample room below the frontend's 130-second request timeout.
		if model.Timeout > 60*time.Second {
			model.Timeout = 60 * time.Second
		}
		ranks, modelErr := a.requestRecommendationAIRanks(ctx, model, s, candidates)
		if modelErr == nil && len(ranks) > 0 {
			ranked := filterRecommendationAIRanks(candidates, ranks)
			for _, candidate := range ranked {
				if candidate.Source == "ai" {
					source = "ai"
					break
				}
			}
			candidates = ranked
		}
		// The rule-ranked candidates remain intact when the model is missing,
		// unavailable, malformed, or returns only IDs outside the whitelist.
	}
	if len(candidates) > 20 {
		candidates = candidates[:20]
	}
	tx, err := a.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	runResult, err := tx.ExecContext(ctx, `insert ignore into biz_recommendation_runs (subscription_id,run_key,source) values (?,?,?)`, s.ID, runKey, source)
	if err != nil {
		return nil, err
	}
	affected, err := runResult.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return []map[string]any{}, nil
	}
	runID, err := runResult.LastInsertId()
	if err != nil {
		return nil, err
	}
	notices := 0
	for _, candidate := range candidates {
		isNew := 0
		if weekly {
			isNew = 1
		}
		scenariosJSON, _ := json.Marshal(candidate.Scenarios)
		evidenceJSON, _ := json.Marshal(candidate.Evidence)
		result, execErr := tx.ExecContext(ctx, `insert ignore into biz_recommendation_results
			(subscription_id,run_id,resource_id,resource_name,score,reason,risks,source,is_new,advice,advice_source,scenarios,evidence,evidence_status,advice_resource_hash)
			values (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, s.ID, runID, candidate.ID, candidate.Name, candidate.Score, candidate.Reason, candidate.Risks, candidate.Source, isNew, candidate.Advice, candidate.AdviceSource, string(scenariosJSON), string(evidenceJSON), candidate.EvidenceStatus, candidate.ResourceHash)
		if execErr != nil {
			return nil, execErr
		}
		inserted, execErr := result.RowsAffected()
		if execErr != nil {
			return nil, execErr
		}
		if inserted == 0 {
			// Refresh an existing match's explanation. Interest feedback and the
			// original "new" marker belong to the user and remain untouched.
			if _, execErr = tx.ExecContext(ctx, `update biz_recommendation_results set resource_name=?,score=?,reason=?,risks=?,source=?,advice=?,advice_source=?,scenarios=?,evidence=?,evidence_status=?,advice_resource_hash=? where subscription_id=? and resource_id=?`,
				candidate.Name, candidate.Score, candidate.Reason, candidate.Risks, candidate.Source, candidate.Advice, candidate.AdviceSource, string(scenariosJSON), string(evidenceJSON), candidate.EvidenceStatus, candidate.ResourceHash, s.ID, candidate.ID); execErr != nil {
				return nil, execErr
			}
			continue
		}
		if !weekly || notices >= 5 {
			continue
		}
		resultID, execErr := result.LastInsertId()
		if execErr != nil {
			return nil, execErr
		}
		targetPath := "/business/assistant?subscriptionId=" + strconv.FormatInt(s.ID, 10)
		title := "达人/媒体新推荐"
		if domain == "ip" {
			targetPath = "/business/ip/recommendations?subscriptionId=" + strconv.FormatInt(s.ID, 10)
			title = "IP 新推荐"
		}
		targetPath += "&resultId=" + strconv.FormatInt(resultID, 10)
		_, execErr = tx.ExecContext(ctx, `insert ignore into biz_recommendation_notifications (user_id,domain,result_id,title,description,target_path) values (?,?,?,?,?,?)`, userID, domain, resultID, title, candidate.Name+" 与你的寻找条件相匹配", targetPath)
		if execErr != nil {
			return nil, execErr
		}
		notices++
	}
	if _, err = tx.ExecContext(ctx, `update biz_recommendation_subscriptions set last_run_at=now() where id=? and user_id=? and domain=?`, s.ID, userID, domain); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return a.recommendationResultsData(ctx, userID, domain, s.ID, "all")
}

func (a *app) recommendationResultsList(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Domain         string `json:"domain"`
		SubscriptionID int64  `json:"subscriptionId"`
		View           string `json:"view"`
	}
	if !readRecommendationJSON(w, r, &input) {
		return
	}
	userID, ok := a.recommendationAccess(w, r, input.Domain)
	if !ok {
		return
	}
	if input.View != "" && input.View != "all" && input.View != "new" {
		writeError(w, http.StatusBadRequest, 400, "无效的结果视图")
		return
	}
	list, err := a.recommendationResultsData(r.Context(), userID, input.Domain, input.SubscriptionID, input.View)
	if err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"list": list, "total": len(list)})
}

func (a *app) recommendationResultsData(ctx context.Context, userID int, domain string, subscriptionID int64, view string) ([]map[string]any, error) {
	query := `select r.id,r.subscription_id,r.resource_id,r.resource_name,r.score,r.reason,r.risks,r.source,r.feedback,r.is_new,r.created_at,r.advice,r.advice_source,r.scenarios,r.evidence,r.evidence_status,r.advice_resource_hash from biz_recommendation_results r join biz_recommendation_subscriptions s on s.id=r.subscription_id where s.user_id=? and s.domain=?`
	args := []any{userID, domain}
	if subscriptionID > 0 {
		query += ` and s.id=?`
		args = append(args, subscriptionID)
	}
	if view == "new" {
		query += ` and r.is_new=1 and r.feedback=''`
	}
	query += ` order by r.created_at desc,r.score desc,r.id desc limit 200`
	rows, err := a.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]map[string]any, 0)
	subscriptions := make(map[int64]recommendationSubscription)
	for rows.Next() {
		var id, subID, resourceID int64
		var score, isNew int
		var name, reason, risks, source, feedback, adviceSource, evidenceStatus, resourceHash string
		var advice, scenariosJSON, evidenceJSON sql.NullString
		var created time.Time
		if err := rows.Scan(&id, &subID, &resourceID, &name, &score, &reason, &risks, &source, &feedback, &isNew, &created, &advice, &adviceSource, &scenariosJSON, &evidenceJSON, &evidenceStatus, &resourceHash); err != nil {
			return nil, err
		}
		s, ok := subscriptions[subID]
		if !ok {
			s, err = scanRecommendationSubscription(a.DB().QueryRowContext(ctx, `select `+recommendationSubscriptionColumns+` from biz_recommendation_subscriptions where id=? and user_id=? and domain=?`, subID, userID, domain))
			if err != nil {
				return nil, err
			}
			subscriptions[subID] = s
		}
		resource, current, err := a.recommendationLiveCandidate(ctx, s, resourceID)
		if err != nil {
			return nil, err
		}
		if resource == nil {
			continue
		}
		visibleAdvice := ""
		visibleAdviceSource := ""
		visibleScenarios := []string{}
		visibleEvidence := []recommendationEvidence{}
		visibleEvidenceStatus := "not_searched"
		if resourceHash != "" && resourceHash == recommendationResourceHash(current.Resource) {
			visibleAdvice = advice.String
			visibleAdviceSource = adviceSource
			visibleEvidenceStatus = evidenceStatus
			if scenariosJSON.Valid {
				_ = json.Unmarshal([]byte(scenariosJSON.String), &visibleScenarios)
			}
			if evidenceJSON.Valid {
				_ = json.Unmarshal([]byte(evidenceJSON.String), &visibleEvidence)
			}
			if visibleScenarios == nil {
				visibleScenarios = []string{}
			}
			if visibleEvidence == nil {
				visibleEvidence = []recommendationEvidence{}
			}
		}
		list = append(list, map[string]any{
			"id": id, "subscriptionId": subID, "resourceId": resourceID, "resourceName": current.Name,
			"score": current.Score, "reason": current.Reason, "risks": current.Risks, "source": source,
			"scoreSource": "rule", "advice": visibleAdvice, "adviceSource": visibleAdviceSource,
			"scenarios": visibleScenarios, "evidence": visibleEvidence, "evidenceStatus": visibleEvidenceStatus,
			"isNew": isNew == 1, "feedback": feedback, "createdAt": created.Format("2006-01-02 15:04"), "resource": resource,
		})
	}
	return list, rows.Err()
}

// Saved results are snapshots for deduplication, while the displayed facts
// must always be checked against current resource, cooperation, and criteria
// data. This prevents a revoked product link or changed market from leaving a
// stale recommendation visible until the next scheduled run.
func (a *app) recommendationLiveCandidate(ctx context.Context, s recommendationSubscription, resourceID int64) (map[string]any, recommendationCandidate, error) {
	resource, err := a.recommendationResource(ctx, s.Domain, resourceID)
	if err != nil || resource == nil {
		return nil, recommendationCandidate{}, err
	}
	current := make(map[string]any, len(resource))
	for key, value := range resource {
		current[key] = value
	}
	var candidate recommendationCandidate
	var ok bool
	if s.Domain == "ip" {
		candidate, ok = scoreIPRecommendation(current, deriveIPRecommendationCriteria(s))
	} else {
		candidate, ok = scoreCreatorRecommendation(current, s, deriveCreatorRecommendationCriteria(s))
	}
	if !ok {
		return nil, recommendationCandidate{}, nil
	}
	return resource, candidate, nil
}

func (a *app) recommendationResource(ctx context.Context, domain string, id int64) (map[string]any, error) {
	var query string
	if domain == "ip" {
		query = `select r.id,r.name,r.ip_type as ipType,r.markets,r.audience,r.summary,r.cooperation_status as cooperationStatus,r.currency,r.price_min as priceMin,r.price_max as priceMax,r.license_notes as licenseNotes,(select f.id from biz_ip_files f where f.ip_id=r.id and f.file_kind='visual' order by f.id desc limit 1) as visualFileId from biz_ip_resources r where r.id=? and r.cooperation_status in ('可合作','洽谈中','待评估')`
	} else {
		productExpression := a.recommendationProductExpression(ctx)
		query = `select r.id,r.name,r.avatar_url as avatarUrl,r.resource_type as resourceType,r.country,r.language,r.platform,r.industry,r.category,r.content_types as contentTypes,r.status,r.followers,r.engagement_rate as engagementRate,r.avg_views as avgViews,r.score,r.level,r.risk_level as riskLevel,` + productExpression + `,(select count(*) from biz_cooperations c where c.resource_id=r.id and c.status='已发布' and trim(coalesce(c.final_link,''))<>'') as publishedCooperations,(select coalesce(sum(c.impressions),0) from biz_cooperations c where c.resource_id=r.id and c.status='已发布' and trim(coalesce(c.final_link,''))<>'') as historicalImpressions from biz_resources r where r.id=? and r.status='可合作'`
	}
	rows, err := a.queryMaps(ctx, query, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	resource := rows[0]
	if domain == "ip" {
		var markets []string
		_ = json.Unmarshal([]byte(recommendationString(resource["markets"])), &markets)
		if markets == nil {
			markets = []string{}
		}
		resource["markets"] = markets
		if resource["priceMin"] != nil {
			resource["priceMin"] = floatField(resource, "priceMin")
		}
		if resource["priceMax"] != nil {
			resource["priceMax"] = floatField(resource, "priceMax")
		}
		if resource["visualFileId"] != nil {
			resource["visualFileId"] = intField(resource, "visualFileId")
		}
	} else {
		products := recommendationString(resource["confirmedProducts"])
		if products == "" {
			resource["confirmedProducts"] = []string{}
		} else {
			resource["confirmedProducts"] = strings.Split(products, "、")
		}
		resource["publishedCooperations"] = intField(resource, "publishedCooperations")
		resource["historicalImpressions"] = intField(resource, "historicalImpressions")
		resource["estimatedCost"] = estimateResourceCost(resource)
		resource["estimatedCostCurrency"] = "USD"
	}
	return resource, nil
}

func (a *app) recommendationResultFeedback(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID       int64  `json:"id"`
		Domain   string `json:"domain"`
		Feedback string `json:"feedback"`
	}
	if !readRecommendationJSON(w, r, &input) {
		return
	}
	userID, ok := a.recommendationAccess(w, r, input.Domain)
	if !ok {
		return
	}
	if input.ID <= 0 || (input.Feedback != "interested" && input.Feedback != "not_interested") {
		writeError(w, http.StatusBadRequest, 400, "反馈内容不合法")
		return
	}
	result, err := a.DB().ExecContext(r.Context(), `update biz_recommendation_results r join biz_recommendation_subscriptions s on s.id=r.subscription_id set r.feedback=?,r.is_new=0 where r.id=? and s.user_id=? and s.domain=?`, input.Feedback, input.ID, userID, input.Domain)
	if err != nil {
		writeDBError(w, err)
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		var exists int
		err = a.DB().QueryRowContext(r.Context(), `select count(*) from biz_recommendation_results r join biz_recommendation_subscriptions s on s.id=r.subscription_id where r.id=? and s.user_id=? and s.domain=?`, input.ID, userID, input.Domain).Scan(&exists)
		if err != nil {
			writeDBError(w, err)
			return
		}
		if exists == 0 {
			writeError(w, http.StatusNotFound, 404, "推荐结果不存在")
			return
		}
	}
	writeOK(w, map[string]any{"id": input.ID})
}

func (a *app) recommendationVisibleDomains(ctx context.Context, userID int) ([]string, error) {
	domains := make([]string, 0, 2)
	for _, item := range []struct{ domain, path string }{{"creator", "/business/assistant"}, {"ip", "/business/ip/recommendations"}} {
		allowed, err := a.hasMenu(ctx, userID, item.path)
		if err != nil {
			return nil, err
		}
		if allowed {
			domains = append(domains, item.domain)
		}
	}
	return domains, nil
}

func (a *app) recommendationNotifications(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, 401, "未登录或登录已过期")
		return
	}
	domains, err := a.recommendationVisibleDomains(r.Context(), userID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if len(domains) == 0 {
		writeOK(w, map[string]any{"list": []map[string]any{}, "unread": 0})
		return
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(domains)), ",")
	args := []any{userID}
	for _, domain := range domains {
		args = append(args, domain)
	}
	rows, err := a.DB().QueryContext(r.Context(), `select n.id,n.domain,n.title,n.description,n.target_path,n.read_at,n.created_at,r.subscription_id,r.resource_id
		from biz_recommendation_notifications n
		join biz_recommendation_results r on r.id=n.result_id
		join biz_recommendation_subscriptions s on s.id=r.subscription_id and s.user_id=n.user_id and s.domain=n.domain
		where n.user_id=? and n.domain in (`+placeholders+`) order by n.created_at desc,n.id desc`, args...)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer rows.Close()
	list := make([]map[string]any, 0)
	subscriptions := make(map[int64]recommendationSubscription)
	unread := 0
	for rows.Next() {
		var id, subscriptionID, resourceID int64
		var domain, title, description, targetPath string
		var readAt sql.NullTime
		var created time.Time
		if err := rows.Scan(&id, &domain, &title, &description, &targetPath, &readAt, &created, &subscriptionID, &resourceID); err != nil {
			writeDBError(w, err)
			return
		}
		s, ok := subscriptions[subscriptionID]
		if !ok {
			s, err = scanRecommendationSubscription(a.DB().QueryRowContext(r.Context(), `select `+recommendationSubscriptionColumns+` from biz_recommendation_subscriptions where id=? and user_id=? and domain=?`, subscriptionID, userID, domain))
			if err != nil {
				writeDBError(w, err)
				return
			}
			subscriptions[subscriptionID] = s
		}
		_, current, err := a.recommendationLiveCandidate(r.Context(), s, resourceID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		if current.ID == 0 {
			continue
		}
		if !readAt.Valid {
			unread++
		}
		if len(list) >= 50 {
			continue
		}
		var read any
		if readAt.Valid {
			read = readAt.Time.Format("2006-01-02 15:04")
		}
		list = append(list, map[string]any{"id": id, "domain": domain, "title": title, "description": current.Name + " 与你的寻找条件相匹配", "targetPath": targetPath, "readAt": read, "createdAt": created.Format("2006-01-02 15:04")})
	}
	if err := rows.Err(); err != nil {
		writeDBError(w, err)
		return
	}
	writeOK(w, map[string]any{"list": list, "unread": unread})
}

func (a *app) recommendationNotificationsRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := a.currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, 401, "未登录或登录已过期")
		return
	}
	var input struct {
		IDs []int64 `json:"ids"`
	}
	if !readRecommendationJSON(w, r, &input) {
		return
	}
	domains, err := a.recommendationVisibleDomains(r.Context(), userID)
	if err != nil {
		writeDBError(w, err)
		return
	}
	if len(domains) == 0 {
		writeOK(w, map[string]any{"updated": 0})
		return
	}
	if len(input.IDs) == 0 || len(input.IDs) > 100 {
		writeError(w, http.StatusBadRequest, 400, "请选择1至100条通知")
		return
	}
	for _, id := range input.IDs {
		if id <= 0 {
			writeError(w, http.StatusBadRequest, 400, "通知编号不合法")
			return
		}
	}
	domainSlots := strings.TrimSuffix(strings.Repeat("?,", len(domains)), ",")
	idSlots := strings.TrimSuffix(strings.Repeat("?,", len(input.IDs)), ",")
	args := []any{userID}
	for _, domain := range domains {
		args = append(args, domain)
	}
	for _, id := range input.IDs {
		args = append(args, id)
	}
	result, err := a.DB().ExecContext(r.Context(), `update biz_recommendation_notifications set read_at=now() where user_id=? and domain in (`+domainSlots+`) and id in (`+idSlots+`) and read_at is null`, args...)
	if err != nil {
		writeDBError(w, err)
		return
	}
	n, _ := result.RowsAffected()
	writeOK(w, map[string]any{"updated": n})
}

func recommendationWeeklyRunKey(now time.Time) (string, bool) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return "", false
	}
	local := now.In(location)
	if local.Weekday() != time.Monday || local.Hour() < 9 {
		return "", false
	}
	return "weekly-" + local.Format("20060102"), true
}

// runAIRecommendationScheduler may run in several server replicas. A
// subscription/run_key unique row is claimed in the same transaction as
// results and notices, preventing duplicate weekly push notifications.
func (a *app) runAIRecommendationScheduler(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	a.runDueAIRecommendations(ctx, time.Now())
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			a.runDueAIRecommendations(ctx, now)
		}
	}
}

func (a *app) runDueAIRecommendations(ctx context.Context, now time.Time) {
	runKey, due := recommendationWeeklyRunKey(now)
	if !due {
		return
	}
	rows, err := a.DB().QueryContext(ctx, `select id,user_id,domain from biz_recommendation_subscriptions where active=1 order by id`)
	if err != nil {
		log.Printf("recommendation scheduler list failed: %v", err)
		return
	}
	type dueSubscription struct {
		ID     int64
		UserID int
		Domain string
	}
	var subscriptions []dueSubscription
	for rows.Next() {
		var s dueSubscription
		if err := rows.Scan(&s.ID, &s.UserID, &s.Domain); err != nil {
			log.Printf("recommendation scheduler scan failed: %v", err)
			break
		}
		subscriptions = append(subscriptions, s)
	}
	if err := rows.Err(); err != nil {
		log.Printf("recommendation scheduler rows failed: %v", err)
	}
	_ = rows.Close()
	for _, s := range subscriptions {
		select {
		case <-ctx.Done():
			return
		default:
		}
		path := "/business/assistant"
		if s.Domain == "ip" {
			path = "/business/ip/recommendations"
		} else if s.Domain != "creator" {
			continue
		}
		allowed, err := a.hasMenu(ctx, s.UserID, path)
		if err != nil {
			log.Printf("recommendation scheduler permission lookup failed: %v", err)
			continue
		}
		if !allowed {
			continue
		}
		runCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		_, err = a.executeRecommendationRun(runCtx, s.ID, s.UserID, s.Domain, runKey, true)
		cancel()
		if err != nil {
			log.Printf("recommendation scheduler run %d failed: %v", s.ID, err)
		}
	}
}
