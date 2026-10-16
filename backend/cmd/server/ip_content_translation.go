package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func ipContentID(row map[string]any, key string) int { id, _ := strconvInt(row[key]); return id }

const ipTranslationEntities = "'ip_resource','ip_request','ip_candidate','ip_case'"

type ipContentTranslation struct {
	State   string                       `json:"state"`
	Total   int                          `json:"total"`
	Pending int                          `json:"pending"`
	Failed  int                          `json:"failed"`
	Values  map[string]map[string]string `json:"values"`
}
type ipContentRecord struct {
	Type        string                `json:"type"`
	ID          int64                 `json:"id"`
	Translation *ipContentTranslation `json:"translation"`
}

func ipTranslationObject(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(anyString(v)), &m)
	return m
}
func ipTranslationValue(row map[string]any, path string) string {
	var v any = row
	for _, part := range strings.Split(path, ".") {
		switch p := v.(type) {
		case []any:
			i, e := strconv.Atoi(part)
			if e != nil || i < 0 || i >= len(p) {
				return ""
			}
			v = p[i]
		default:
			v = ipTranslationObject(v)[part]
		}
	}
	s, _ := v.(string)
	return s
}
func ipTranslationTarget(text string) string {
	if containsHan(text) {
		return "en"
	}
	for _, r := range text {
		if unicode.Is(unicode.Latin, r) {
			return "zh"
		}
	}
	return ""
}

// Explicit narrative fields only: never translate contacts, employee IDs, fixed enums, filenames or chat history.
func ipContentSources(kind string, row map[string]any) []translationSource {
	fields := []string{}
	switch kind {
	case "ip_resource":
		fields = []string{"name", "rightsOwner", "audience", "summary", "licenseNotes"}
		for _, key := range []string{"imageStyle", "lifecycle", "genderRatio", "ageRange", "spendingPower", "registeredUsers", "salesROI", "showcase", "cooperationTips"} {
			fields = append(fields, "profile."+key)
		}
	case "ip_case":
		fields = []string{"title", "summary"}
	case "ip_candidate":
		fields = []string{"reason"}
		for _, key := range []string{"priceModel", "licenseContent", "licenseTerritory", "ipResources", "brandResources", "businessRisk", "publicRisk", "ownershipClarity", "pastCases", "relatedDocuments", "notes"} {
			fields = append(fields, "assessment."+key)
		}
	case "ip_request":
		fields = []string{"projectName", "department", "description", "externalRecommendation", "marketHeat", "fanAudience", "commercialValue", "marketingRisks", "marketingComments"}
		for _, key := range []string{"projectPeriod", "targetIPType", "cooperationMode", "projectNotes"} {
			fields = append(fields, "brief."+key)
		}
		brief := ipTranslationObject(row["brief"])
		for id := range ipTranslationObject(brief["candidateEvaluations"]) {
			if _, err := strconv.ParseInt(id, 10, 64); err == nil {
				for _, key := range []string{"reason", "productEvaluation", "technicalFit"} {
					fields = append(fields, "brief.candidateEvaluations."+id+"."+key)
				}
			}
		}
		external, _ := brief["externalIPs"].([]any)
		for i := range external {
			for _, key := range []string{"name", "rightsOwner", "reason"} {
				fields = append(fields, fmt.Sprintf("brief.externalIPs.%d.%s", i, key))
			}
		}
		profile := ipTranslationObject(row["marketingProfile"])
		for _, role := range ipMarketingFunctions {
			fields = append(fields, "marketingProfile.functions."+role.Key+".comments")
		}
		for id := range ipTranslationObject(profile["candidates"]) {
			if _, err := strconv.ParseInt(id, 10, 64); err == nil {
				for _, role := range ipMarketingFunctions {
					for _, field := range role.Fields {
						fields = append(fields, "marketingProfile.candidates."+id+"."+field)
					}
				}
			}
		}
	}
	sort.Strings(fields)
	sources := []translationSource{}
	for _, path := range fields {
		text := strings.TrimSpace(ipTranslationValue(row, path))
		if text == "" || ipTranslationTarget(text) == "" || strings.HasPrefix(text, "https://") || strings.HasPrefix(text, "http://") {
			continue
		}
		sources = append(sources, translationSource{EntityType: kind, EntityID: int(ipContentID(row, "id")), FieldKey: path, Text: text})
	}
	return sources
}

func (a *app) ipContentRows(ctx context.Context, kind string, id int64) ([]map[string]any, error) {
	switch kind {
	case "ip_resource":
		return a.queryMaps(ctx, `select id,name,rights_owner as rightsOwner,audience,summary,license_notes as licenseNotes,profile from biz_ip_resources where id=?`, id)
	case "ip_request":
		return a.queryMaps(ctx, `select id,project_name as projectName,department,description,external_recommendation as externalRecommendation,brief,marketing_profile as marketingProfile,market_heat as marketHeat,fan_audience as fanAudience,commercial_value as commercialValue,marketing_risks as marketingRisks,marketing_comments as marketingComments from biz_ip_requests where id=?`, id)
	case "ip_case":
		return a.queryMaps(ctx, `select id,title,summary from biz_ip_cases where id=?`, id)
	case "ip_candidate":
		return a.queryMaps(ctx, `select id,reason,assessment from biz_ip_request_candidates where id=?`, id)
	}
	return nil, errors.New("invalid IP translation entity")
}

func (a *app) queueIPContent(ctx context.Context, kind string, ids ...int64) {
	// Existing installations without a model key keep saving normally.
	if strings.TrimSpace(a.Config().AIModel.APIKey) == "" {
		return
	}
	for _, id := range ids {
		if err := a.queueIPContentRecord(ctx, kind, id); err != nil {
			log.Printf("IP translation queue failed for %s %d", kind, id)
		}
	}
}
func (a *app) queueIPContentRecord(ctx context.Context, kind string, id int64) error {
	rows, err := a.ipContentRows(ctx, kind, id)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return sql.ErrNoRows
	}
	sources := ipContentSources(kind, rows[0])
	if kind == "ip_request" {
		candidates, e := a.queryMaps(ctx, "select id,reason,assessment from biz_ip_request_candidates where request_id=?", id)
		if e != nil {
			return e
		}
		for _, row := range candidates {
			sources = append(sources, ipContentSources("ip_candidate", row)...)
		}
	}
	return a.enqueueIPContentSources(ctx, sources)
}
func (a *app) enqueueIPContentSources(ctx context.Context, sources []translationSource) error {
	if len(sources) == 0 {
		return nil
	}
	tx, err := a.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, s := range sources {
		target := ipTranslationTarget(s.Text)
		source := "en"
		if target == "en" {
			source = "zh-CN"
		}
		_, err = tx.ExecContext(ctx, `insert into biz_localized_texts (entity_type,entity_id,field_key,source_language,target_language,source_hash,source_text,translated_text,translation_status,error_message) values (?,?,?,?,?,?,?,'','pending','') on duplicate key update translation_status=if(source_hash=values(source_hash) and translation_status in ('completed','processing'),translation_status,'pending'),translated_text=if(source_hash=values(source_hash) and translation_status='completed',translated_text,''),source_language=values(source_language),source_text=values(source_text),error_message='',source_hash=values(source_hash)`, s.EntityType, s.EntityID, s.FieldKey, source, target, localizedSourceHash(s.Text), s.Text)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (a *app) attachIPContentTranslations(ctx context.Context, kind string, rows []map[string]any) error {
	if len(rows) == 0 || strings.TrimSpace(a.Config().AIModel.APIKey) == "" {
		return nil
	}
	marks := []string{}
	args := []any{kind}
	byID := map[int]map[string]any{}
	meta := map[int]*ipContentTranslation{}
	for _, row := range rows {
		id := ipContentID(row, "id")
		if id <= 0 {
			continue
		}
		marks = append(marks, "?")
		args = append(args, id)
		byID[id] = row
		m := &ipContentTranslation{State: "none", Values: map[string]map[string]string{"en": {}, "zh": {}}}
		meta[id] = m
		row["translation"] = m
	}
	if len(marks) == 0 {
		return nil
	}
	r, err := a.DB().QueryContext(ctx, "select entity_id,field_key,target_language,source_hash,translated_text,translation_status from biz_localized_texts where entity_type=? and entity_id in ("+strings.Join(marks, ",")+")", args...)
	if err != nil {
		return err
	}
	defer r.Close()
	seen := map[string]bool{}
	for r.Next() {
		var id int
		var path, target, hash, text, status string
		if err := r.Scan(&id, &path, &target, &hash, &text, &status); err != nil {
			return err
		}
		source := ipTranslationValue(byID[id], path)
		if strings.TrimSpace(source) == "" || localizedSourceHash(source) != hash || meta[id] == nil {
			continue
		}
		m := meta[id]
		seen[fmt.Sprintf("%d:%s", id, path)] = true
		m.Total++
		switch status {
		case "completed":
			if text != "" && m.Values[target] != nil {
				m.Values[target][path] = text
			} else {
				m.Failed++
			}
		case "failed":
			m.Failed++
		default:
			m.Pending++
		}
	}
	if err := r.Err(); err != nil {
		return err
	}
	for id, row := range byID {
		m := meta[id]
		for _, s := range ipContentSources(kind, row) {
			if !seen[fmt.Sprintf("%d:%s", id, s.FieldKey)] {
				m.Total++
				m.Failed++
			}
		}
		switch {
		case m.Pending > 0:
			m.State = "pending"
		case m.Failed > 0:
			m.State = "failed"
		case m.Total > 0:
			m.State = "completed"
		}
	}
	return nil
}

func (a *app) runIPContentTranslationWorker(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		if err := a.processIPContentTranslations(ctx); err != nil {
			log.Printf("IP translation worker temporarily unavailable")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (a *app) processIPContentTranslations(ctx context.Context) error {
	if strings.TrimSpace(a.Config().AIModel.APIKey) == "" {
		return nil
	}
	rows, err := a.queryMaps(ctx, "select id,entity_type as entityType,entity_id as entityId,field_key as fieldKey,target_language as target,source_text as text,source_hash as hash from biz_localized_texts where entity_type in ("+ipTranslationEntities+") and (translation_status='pending' or (translation_status='processing' and updated_at<date_sub(now(),interval 5 minute))) order by updated_at,id limit 24")
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	model, configured := a.assistantAIModel(ctx)
	current := map[string]map[string]translationSource{}
	for _, row := range rows {
		key := fmt.Sprintf("%s:%d", str(row, "entityType"), ipContentID(row, "entityId"))
		if _, ok := current[key]; ok {
			continue
		}
		items, err := a.ipContentRows(ctx, str(row, "entityType"), int64(ipContentID(row, "entityId")))
		if err != nil {
			return err
		}
		current[key] = map[string]translationSource{}
		if len(items) > 0 {
			for _, source := range ipContentSources(str(row, "entityType"), items[0]) {
				current[key][source.FieldKey] = source
			}
		}
	}
	for _, target := range []string{"en", "zh"} {
		claimed := []map[string]any{}
		sources := []translationSource{}
		chars := 0
		for _, row := range rows {
			if str(row, "target") != target {
				continue
			}
			key := fmt.Sprintf("%s:%d", str(row, "entityType"), ipContentID(row, "entityId"))
			fresh, exists := current[key][str(row, "fieldKey")]
			if !exists || localizedSourceHash(fresh.Text) != str(row, "hash") || ipTranslationTarget(fresh.Text) != target {
				// Deleted records/removed text are not sent to the model. An edit made while queuing wins.
				if _, err := a.DB().ExecContext(ctx, "delete from biz_localized_texts where id=? and source_hash=?", row["id"], row["hash"]); err != nil {
					return err
				}
				if exists {
					if err := a.enqueueIPContentSources(ctx, []translationSource{fresh}); err != nil {
						return err
					}
				}
				continue
			}
			text := str(row, "text")
			if chars+len(text) > 32000 && len(claimed) > 0 {
				break
			}
			result, err := a.DB().ExecContext(ctx, "update biz_localized_texts set translation_status='processing',updated_at=current_timestamp where id=? and source_hash=? and (translation_status='pending' or (translation_status='processing' and updated_at<date_sub(now(),interval 5 minute)))", row["id"], row["hash"])
			if err != nil {
				return err
			}
			n, _ := result.RowsAffected()
			if n == 0 {
				continue
			}
			claimed = append(claimed, row)
			sources = append(sources, translationSource{FieldKey: str(row, "fieldKey"), Text: text})
			chars += len(text)
		}
		if len(claimed) == 0 {
			continue
		}
		var translated map[int]string
		var requestErr error
		if configured {
			callCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
			translated, requestErr = requestAITranslations(callCtx, model, sources, target)
			cancel()
		} else {
			requestErr = errors.New("model unavailable")
		}
		if len(translated) != len(claimed) {
			requestErr = errors.New("incomplete translation")
		}
		for i, row := range claimed {
			text := strings.TrimSpace(translated[i])
			status := "completed"
			message := ""
			if requestErr != nil || text == "" || len(text) > 60000 {
				status = "failed"
				message = "AI translation unavailable; retry later"
				text = ""
			}
			// A completion for an older version must never overwrite a newer pending edit.
			_, err = a.DB().ExecContext(ctx, "update biz_localized_texts set translated_text=?,translation_status=?,error_message=?,updated_at=current_timestamp where id=? and source_hash=? and translation_status='processing'", text, status, message, row["id"], row["hash"])
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *app) ipContentTranslationStatus(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Type  string `json:"type"`
		ID    int64  `json:"id"`
		Retry bool   `json:"retry"`
	}
	if !readIPJSON(w, r, &input) {
		return
	}
	if input.ID <= 0 || (input.Type != "ip_resource" && input.Type != "ip_request") {
		writeError(w, 400, 400, "无效的翻译记录")
		return
	}
	required := []string{"/business/ip/requests", "/business/ip/feedback", "/business/ip/marketing", "/business/ip/recommendations"}
	if input.Type == "ip_resource" {
		required = append(required, "/business/ip/resources")
	}
	a.requireAnyMenu(required, func(w http.ResponseWriter, r *http.Request) {
		rows, err := a.ipContentRows(r.Context(), input.Type, input.ID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		if len(rows) == 0 {
			writeError(w, 404, 404, "记录不存在")
			return
		}
		if input.Retry {
			a.queueIPContent(r.Context(), input.Type, input.ID)
		}
		records := []ipContentRecord{}
		add := func(kind string, items []map[string]any) bool {
			if err := a.attachIPContentTranslations(r.Context(), kind, items); err != nil {
				writeDBError(w, err)
				return false
			}
			for _, row := range items {
				m, _ := row["translation"].(*ipContentTranslation)
				if m == nil {
					m = &ipContentTranslation{State: "unavailable"}
				}
				records = append(records, ipContentRecord{kind, int64(ipContentID(row, "id")), m})
			}
			return true
		}
		if !add(input.Type, rows) {
			return
		}
		if input.Type == "ip_request" {
			c, e := a.queryMaps(r.Context(), "select id,reason,assessment from biz_ip_request_candidates where request_id=?", input.ID)
			if e != nil {
				writeDBError(w, e)
				return
			}
			if !add("ip_candidate", c) {
				return
			}
		} else {
			c, e := a.queryMaps(r.Context(), "select id,title,summary from biz_ip_cases where ip_id=?", input.ID)
			if e != nil {
				writeDBError(w, e)
				return
			}
			if input.Retry {
				for _, row := range c {
					a.queueIPContent(r.Context(), "ip_case", int64(ipContentID(row, "id")))
				}
			}
			if !add("ip_case", c) {
				return
			}
		}
		writeOK(w, records)
	})(w, r)
}
