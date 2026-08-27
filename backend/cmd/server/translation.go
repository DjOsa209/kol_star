package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"
)

const englishLocale = "en"

type translationSource struct {
	EntityType string
	EntityID   int
	FieldKey   string
	Text       string
}

type translationSummary struct {
	Translated int `json:"translated"`
	Cached     int `json:"cached"`
	Skipped    int `json:"skipped"`
}

var resourceTranslationColumns = []struct {
	Column string
	Key    string
}{
	{"title", "title"},
	{"industry", "industry"},
	{"category", "category"},
	{"country", "country"},
	{"region", "region"},
	{"city", "city"},
	{"language", "language"},
	{"content_types", "contentTypes"},
	{"notes", "notes"},
}

var englishTranslationGlossary = map[string]string{
	"可合作": "Available for Collaboration", "暂停合作": "Collaboration Paused", "已停用": "Disabled",
	"媒体": "Media", "艺术家": "Artist", "代理商": "Agency",
	"科技": "Technology", "生活方式": "Lifestyle", "商业": "Business", "设计": "Design",
	"游戏": "Gaming", "摄影": "Photography", "体育": "Sports", "娱乐": "Entertainment",
	"汽车": "Automotive", "财经": "Finance", "教育": "Education", "大众媒体": "Mass Media",
	"头部": "Top-tier", "腰部": "Mid-tier", "尾部": "Long-tail",
	"付费合作": "Paid Collaboration", "产品置换": "Product Exchange", "联盟合作": "Affiliate Collaboration",
	"活动合作": "Event Collaboration", "采访合作": "Interview Collaboration",
	"低": "Low", "中": "Medium", "高": "High", "启用": "Enabled", "停用": "Disabled",
}

func containsHan(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func localizedSourceHash(text string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(text)))
	return hex.EncodeToString(sum[:])
}

func normalizeTargetLanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "en", "en-us", "en_us", "english":
		return englishLocale
	default:
		return ""
	}
}

func (a *app) translateBusinessResources(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ids := intSliceField(body, "ids")
	if len(ids) == 0 {
		writeError(w, http.StatusOK, 10001, "请选择要翻译的资源")
		return
	}
	if len(ids) > 100 {
		writeError(w, http.StatusOK, 10001, "单次最多翻译 100 条资源")
		return
	}
	target := normalizeTargetLanguage(str(body, "targetLanguage"))
	if target == "" {
		target = englishLocale
	}
	summary, err := a.translateResourceIDs(r.Context(), ids, target)
	if err != nil {
		writeError(w, http.StatusOK, 10006, err.Error())
		return
	}
	writeOK(w, summary)
}

func (a *app) scheduleResourceTranslations(ids []int) {
	if len(ids) == 0 {
		return
	}
	ids = append([]int(nil), ids...)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if _, err := a.translateResourceIDs(ctx, ids, englishLocale); err != nil {
			log.Printf("translate imported resources: %v", err)
		}
	}()
}

func (a *app) translateResourceIDs(ctx context.Context, ids []int, target string) (translationSummary, error) {
	var summary translationSummary
	sources, err := a.resourceTranslationSources(ctx, ids)
	if err != nil {
		return summary, err
	}
	pending := make([]translationSource, 0, len(sources))
	for _, source := range sources {
		source.Text = strings.TrimSpace(source.Text)
		if source.Text == "" || !containsHan(source.Text) {
			summary.Skipped++
			continue
		}
		cached, err := a.translationIsCurrent(ctx, source, target)
		if err != nil {
			return summary, err
		}
		if cached {
			summary.Cached++
			continue
		}
		if translated, ok := englishTranslationGlossary[source.Text]; target == englishLocale && ok {
			if err := a.saveTranslation(ctx, source, target, translated); err != nil {
				return summary, err
			}
			summary.Translated++
			continue
		}
		pending = append(pending, source)
	}
	if len(pending) == 0 {
		return summary, nil
	}
	model, ok := a.assistantAIModel(ctx)
	if !ok {
		return summary, fmt.Errorf("AI 翻译尚未配置；请先在数据治理中启用并配置 AI 模型")
	}
	for start := 0; start < len(pending); start += 40 {
		end := start + 40
		if end > len(pending) {
			end = len(pending)
		}
		translated, err := requestAITranslations(ctx, model, pending[start:end], target)
		if err != nil {
			return summary, err
		}
		for index, source := range pending[start:end] {
			text := strings.TrimSpace(translated[index])
			if text == "" {
				summary.Skipped++
				continue
			}
			if err := a.saveTranslation(ctx, source, target, text); err != nil {
				return summary, err
			}
			summary.Translated++
		}
	}
	return summary, nil
}

func (a *app) resourceTranslationSources(ctx context.Context, ids []int) ([]translationSource, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i], args[i] = "?", id
	}
	columns := []string{"id"}
	for _, field := range resourceTranslationColumns {
		columns = append(columns, field.Column)
	}
	rows, err := a.DB().QueryContext(ctx,
		"select "+strings.Join(columns, ",")+" from biz_resources where id in ("+strings.Join(placeholders, ",")+")",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]translationSource, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		id, _ := strconvInt(values[0])
		for i, field := range resourceTranslationColumns {
			result = append(result, translationSource{EntityType: "resource", EntityID: id, FieldKey: field.Key, Text: translationDBText(values[i+1])})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	extraRows, err := a.DB().QueryContext(ctx,
		`select v.resource_id, f.id, f.field_key, f.label, v.value
		   from biz_resource_extra_values v
		   join biz_resource_extra_fields f on f.id = v.field_id
		  where v.resource_id in (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer extraRows.Close()
	seenLabels := map[int]bool{}
	for extraRows.Next() {
		var resourceID, fieldID int
		var fieldKey, label, value string
		if err := extraRows.Scan(&resourceID, &fieldID, &fieldKey, &label, &value); err != nil {
			return nil, err
		}
		if !seenLabels[fieldID] {
			result = append(result, translationSource{EntityType: "resource_extra_field", EntityID: fieldID, FieldKey: "label", Text: label})
			seenLabels[fieldID] = true
		}
		result = append(result, translationSource{EntityType: "resource_extra_value", EntityID: resourceID, FieldKey: fieldKey, Text: value})
	}
	if err := extraRows.Err(); err != nil {
		return nil, err
	}
	postRows, err := a.DB().QueryContext(ctx,
		`select id, title, description
		   from biz_resource_platform_posts
		  where resource_id in (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer postRows.Close()
	for postRows.Next() {
		var id int
		var title, description sql.NullString
		if err := postRows.Scan(&id, &title, &description); err != nil {
			return nil, err
		}
		result = append(result,
			translationSource{EntityType: "resource_post", EntityID: id, FieldKey: "title", Text: title.String},
			translationSource{EntityType: "resource_post", EntityID: id, FieldKey: "description", Text: description.String},
		)
	}
	return result, postRows.Err()
}

func translationDBText(value any) string {
	if bytes, ok := value.([]byte); ok {
		return strings.TrimSpace(string(bytes))
	}
	return anyString(value)
}

func strconvInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int64:
		return int(typed), true
	case []byte:
		var id int
		_, err := fmt.Sscan(string(typed), &id)
		return id, err == nil
	default:
		var id int
		_, err := fmt.Sscan(fmt.Sprint(value), &id)
		return id, err == nil
	}
}

func (a *app) translationIsCurrent(ctx context.Context, source translationSource, target string) (bool, error) {
	var count int
	err := a.DB().QueryRowContext(ctx,
		`select count(*) from biz_localized_texts
		  where entity_type = ? and entity_id = ? and field_key = ? and target_language = ?
		    and source_hash = ? and translation_status = 'completed'`,
		source.EntityType, source.EntityID, source.FieldKey, target, localizedSourceHash(source.Text),
	).Scan(&count)
	return count > 0, err
}

func (a *app) saveTranslation(ctx context.Context, source translationSource, target, translated string) error {
	_, err := a.DB().ExecContext(ctx,
		`insert into biz_localized_texts
		 (entity_type, entity_id, field_key, source_language, target_language, source_hash, source_text, translated_text, translation_status, error_message)
		 values (?, ?, ?, 'zh-CN', ?, ?, ?, ?, 'completed', '')
		 on duplicate key update source_hash = values(source_hash), source_text = values(source_text),
		 translated_text = values(translated_text), translation_status = 'completed', error_message = '', updated_at = current_timestamp`,
		source.EntityType, source.EntityID, source.FieldKey, target, localizedSourceHash(source.Text), source.Text, translated,
	)
	return err
}

func requestAITranslations(ctx context.Context, model assistantAIModel, sources []translationSource, target string) (map[int]string, error) {
	items := make([]map[string]any, 0, len(sources))
	for index, source := range sources {
		items = append(items, map[string]any{"id": index, "field": source.FieldKey, "text": source.Text})
	}
	payload, _ := json.Marshal(map[string]any{
		"sourceLanguage": "zh-CN", "targetLanguage": target, "items": items,
		"glossary": map[string]string{"达人": "creator", "种草": "product recommendation", "腰部达人": "mid-tier creator", "投放": "campaign placement"},
	})
	requestBody, _ := json.Marshal(map[string]any{
		"model": model.Model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a professional localization engine for a global creator and media resource platform. Translate Chinese text into concise, natural business English. Preserve names, handles, URLs, email addresses, brands, product names, numbers and JSON structure. Return strict JSON only: {\"translations\":[{\"id\":0,\"text\":\"...\"}]}. Return exactly one item for every input id."},
			{"role": "user", "content": string(payload)},
		},
		"temperature":     0.1,
		"response_format": map[string]string{"type": "json_object"},
	})
	endpoint := strings.TrimRight(model.BaseURL, "/")
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("AI 翻译服务返回异常：%s", strings.TrimSpace(string(body)))
	}
	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return nil, err
	}
	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("AI 翻译服务未返回内容")
	}
	var decoded struct {
		Translations []struct {
			ID   int    `json:"id"`
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err := json.Unmarshal([]byte(cleanJSONContent(completion.Choices[0].Message.Content)), &decoded); err != nil {
		return nil, fmt.Errorf("AI 翻译结果解析失败：%w", err)
	}
	result := make(map[int]string, len(decoded.Translations))
	for _, item := range decoded.Translations {
		result[item.ID] = item.Text
	}
	return result, nil
}

func (a *app) attachLocalizedResourceText(ctx context.Context, rows []map[string]any, target string) error {
	if len(rows) == 0 || target == "" {
		return nil
	}
	ids := make([]int, 0, len(rows))
	byID := make(map[int]map[string]any, len(rows))
	for _, row := range rows {
		id := intField(row, "id")
		if id == 0 {
			continue
		}
		ids = append(ids, id)
		byID[id] = row
		row["localized"] = map[string]string{}
		row["localizedExtraFields"] = map[string]string{}
	}
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+1)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, target)
	translations, err := a.DB().QueryContext(ctx,
		`select entity_type, entity_id, field_key, source_hash, translated_text
		   from biz_localized_texts
		  where entity_type in ('resource', 'resource_extra_value')
		    and entity_id in (`+strings.Join(placeholders, ",")+`)
		    and target_language = ? and translation_status = 'completed'`, args...)
	if err != nil {
		return err
	}
	defer translations.Close()
	for translations.Next() {
		var entityType, fieldKey, sourceHash, translated string
		var entityID int
		if err := translations.Scan(&entityType, &entityID, &fieldKey, &sourceHash, &translated); err != nil {
			return err
		}
		row := byID[entityID]
		if row == nil {
			continue
		}
		if entityType == "resource" {
			if localizedSourceHash(anyString(row[fieldKey])) != sourceHash {
				continue
			}
			row["localized"].(map[string]string)[fieldKey] = translated
			continue
		}
		extra, _ := row["extraFields"].(map[string]any)
		if localizedSourceHash(anyString(extra[fieldKey])) == sourceHash {
			row["localizedExtraFields"].(map[string]string)[fieldKey] = translated
		}
	}
	return translations.Err()
}

func (a *app) attachLocalizedExtraFieldLabels(ctx context.Context, rows []map[string]any, target string) error {
	for _, row := range rows {
		id := intField(row, "id")
		label := anyString(row["label"])
		if id == 0 || label == "" {
			continue
		}
		var sourceHash, translated string
		err := a.DB().QueryRowContext(ctx,
			`select source_hash, translated_text from biz_localized_texts
			  where entity_type = 'resource_extra_field' and entity_id = ? and field_key = 'label'
			    and target_language = ? and translation_status = 'completed' limit 1`, id, target,
		).Scan(&sourceHash, &translated)
		if err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return err
		}
		if localizedSourceHash(label) == sourceHash {
			row["localizedLabel"] = translated
		}
	}
	return nil
}

func (a *app) attachLocalizedPostText(ctx context.Context, rows []map[string]any, target string) error {
	if len(rows) == 0 || target == "" {
		return nil
	}
	ids := make([]int, 0, len(rows))
	byID := make(map[int]map[string]any, len(rows))
	for _, row := range rows {
		id := intField(row, "id")
		if id == 0 {
			continue
		}
		ids = append(ids, id)
		byID[id] = row
		row["localized"] = map[string]string{}
	}
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+1)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, target)
	translations, err := a.DB().QueryContext(ctx,
		`select entity_id, field_key, source_hash, translated_text
		   from biz_localized_texts
		  where entity_type = 'resource_post'
		    and entity_id in (`+strings.Join(placeholders, ",")+`)
		    and target_language = ? and translation_status = 'completed'`, args...)
	if err != nil {
		return err
	}
	defer translations.Close()
	for translations.Next() {
		var entityID int
		var fieldKey, sourceHash, translated string
		if err := translations.Scan(&entityID, &fieldKey, &sourceHash, &translated); err != nil {
			return err
		}
		row := byID[entityID]
		if row != nil && localizedSourceHash(anyString(row[fieldKey])) == sourceHash {
			row["localized"].(map[string]string)[fieldKey] = translated
		}
	}
	return translations.Err()
}

func sortedUniqueInts(values []int) []int {
	seen := map[int]bool{}
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value > 0 && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Ints(result)
	return result
}
