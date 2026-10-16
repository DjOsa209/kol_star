package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

//go:embed data/ip_en.json
var ipEnglishCatalogJSON []byte
var ipEnglishCatalog = func() map[string]string {
	var values map[string]string
	if err := json.Unmarshal(ipEnglishCatalogJSON, &values); err != nil {
		panic(err)
	}
	return values
}()

type ipLanguageKey struct{}

func ipLanguage(ctx context.Context) string {
	language, _ := ctx.Value(ipLanguageKey{}).(string)
	if language == "en" {
		return "en"
	}
	return "zh"
}
func ipRequestLanguage(r *http.Request) string {
	value := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Accept-Language"), ",")[0]))
	if strings.HasPrefix(value, "en") {
		return "en"
	}
	return "zh"
}
func ipLanguageContext(r *http.Request) context.Context {
	return context.WithValue(r.Context(), ipLanguageKey{}, ipRequestLanguage(r))
}

type ipLocalizedWriter struct {
	http.ResponseWriter
	language string
}

func (w *ipLocalizedWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (a *app) withIPLocale(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/business/ip/") || strings.HasPrefix(r.URL.Path, "/business/recommendations/") {
			r = r.WithContext(ipLanguageContext(r))
			w = &ipLocalizedWriter{w, ipLanguage(r.Context())}
		}
		next.ServeHTTP(w, r)
	})
}
func ipResponseLanguage(w http.ResponseWriter) string {
	if v, ok := w.(*ipLocalizedWriter); ok {
		return v.language
	}
	return "zh"
}
func ipLoggingWriter(w http.ResponseWriter) *loggingResponseWriter {
	for {
		if logger, ok := w.(*loggingResponseWriter); ok {
			return logger
		}
		if wrapper, ok := w.(interface{ Unwrap() http.ResponseWriter }); ok {
			w = wrapper.Unwrap()
		} else {
			return nil
		}
	}
}

type ipLocalePattern struct {
	source *regexp.Regexp
	target string
}

var ipLocaleToken = regexp.MustCompile(`\{\d+\}|%[+\-#.0-9]*[sdvf]`)
var ipLocalePatterns = func() []ipLocalePattern {
	keys := []string{}
	for source := range ipEnglishCatalog {
		if ipLocaleToken.MatchString(source) {
			keys = append(keys, source)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	patterns := []ipLocalePattern{}
	for _, source := range keys {
		indices := ipLocaleToken.FindAllStringIndex(source, -1)
		var expression strings.Builder
		expression.WriteString("^")
		offset := 0
		for _, v := range indices {
			expression.WriteString(regexp.QuoteMeta(source[offset:v[0]]))
			expression.WriteString("(.*?)")
			offset = v[1]
		}
		expression.WriteString(regexp.QuoteMeta(source[offset:]))
		expression.WriteString("$")
		patterns = append(patterns, ipLocalePattern{regexp.MustCompile("(?s)" + expression.String()), ipEnglishCatalog[source]})
	}
	return patterns
}()

func ipEnglishText(source string) string {
	if translated, ok := ipEnglishCatalog[source]; ok {
		return translated
	}
	for _, pattern := range ipLocalePatterns {
		captures := pattern.source.FindStringSubmatch(source)
		if captures == nil {
			continue
		}
		index := 0
		return ipLocaleToken.ReplaceAllStringFunc(pattern.target, func(token string) string {
			position := index
			index++
			if strings.HasPrefix(token, "{") {
				fmt.Sscanf(token, "{%d}", &position)
			}
			if position+1 < len(captures) {
				return captures[position+1]
			}
			return token
		})
	}
	for _, separator := range []string{"；", " / ", "、", "\n"} {
		if strings.Contains(source, separator) {
			parts := strings.Split(source, separator)
			changed := false
			for i, part := range parts {
				translated := ipEnglishText(part)
				changed = changed || translated != part
				parts[i] = translated
			}
			if changed {
				return strings.Join(parts, map[string]string{"；": "; ", "、": ", ", " / ": " / ", "\n": "\n"}[separator])
			}
		}
	}
	return source
}
func ipLocaleText(ctx context.Context, source string) string {
	if ipLanguage(ctx) == "en" {
		return ipEnglishText(source)
	}
	return source
}
func ipModelLanguageInstruction(ctx context.Context) string {
	if ipLanguage(ctx) != "en" {
		return ""
	}
	return `
LANGUAGE RULE (takes precedence over Chinese examples above): Write all NEW user-facing recommendation narratives, headlines, explanations, questions, reasons, advice, basis labels/values, pending items, and suggested follow-up prompts in natural English. Advice must use the English section labels "Use case:" and "Why it fits:". Keep JSON field names and canonical filter values (including Chinese ipTypes/markets/kind enums) exactly as supplied; never translate identifiers. Preserve actual IP names and verbatim evidence quotes in their original language. Do not rewrite historical conversation or user-entered business data.`
}
func localizeIPPresentation(ctx context.Context, p ipRecommendationPresentation) ipRecommendationPresentation {
	if ipLanguage(ctx) != "en" {
		return p
	}
	p.Headline = ipEnglishText(p.Headline)
	p.Explanation = ipEnglishText(p.Explanation)
	p.Basis = append([]ipRecommendationBasis{}, p.Basis...)
	for i := range p.Basis {
		p.Basis[i].Label = ipEnglishText(p.Basis[i].Label)
		p.Basis[i].Value = ipEnglishText(p.Basis[i].Value)
	}
	p.Adjustments = ipEnglishList(p.Adjustments)
	p.Questions = ipEnglishList(p.Questions)
	p.SuggestedPrompts = ipEnglishList(p.SuggestedPrompts)
	if p.WebSearch != nil {
		copy := *p.WebSearch
		copy.Gaps = ipEnglishList(copy.Gaps)
		copy.Candidates = append([]ipExternalCandidate{}, copy.Candidates...)
		for i := range copy.Candidates {
			copy.Candidates[i].Pending = ipEnglishList(copy.Candidates[i].Pending)
			copy.Candidates[i].EntryPoints = ipEnglishList(copy.Candidates[i].EntryPoints)
		}
		p.WebSearch = &copy
	}
	return p
}
func ipEnglishList(values []string) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = ipEnglishText(value)
	}
	return out
}

// Only generated metadata is localized. Business records and stored chat history remain untouched.
func ipLocalizeMetadata(value any) any {
	switch item := value.(type) {
	case tableData:
		item.List = ipLocalizeMetadata(item.List)
		return item
	case map[string]ipStageDeadline:
		out := make(map[string]ipStageDeadline, len(item))
		for key, deadline := range item {
			deadline.Label = ipEnglishText(deadline.Label)
			deadline.Warning = ipEnglishText(deadline.Warning)
			out[key] = deadline
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(item))
		for key, v := range item {
			switch key {
			case "message", "warning":
				if text, ok := v.(string); ok {
					out[key] = ipEnglishText(text)
				} else {
					out[key] = v
				}
			case "deadlines", "request":
				out[key] = ipLocalizeMetadata(v)
			case "ip", "marketing":
				out[key] = ipLocalizeMetadata(v)
			default:
				out[key] = v
			}
		}
		return out
	case []map[string]any:
		out := make([]map[string]any, len(item))
		for i, row := range item {
			out[i] = ipLocalizeMetadata(row).(map[string]any)
		}
		return out
	default:
		return value
	}
}

func ipEnglishCandidate(s recommendationSubscription, c recommendationCandidate) recommendationCandidate {
	c.Reason = ipEnglishText(c.Reason)
	c.Risks = ipEnglishText(c.Risks)
	c.Scenarios = ipEnglishList(c.Scenarios)
	rights := "Confirm territorial rights, permitted assets, channels, pricing and availability with the rights holder."
	if c.AdviceSource != "ai" {
		scenario := "Assess a product launch collaboration"
		if len(c.Scenarios) > 0 {
			scenario = c.Scenarios[0]
		}
		c.Advice = fmt.Sprintf("Use case: %s. Why it fits: %s. %s", scenario, c.Reason, rights)
		if c.Assessment != nil {
			a := *c.Assessment
			a.Verdict = fmt.Sprintf("Assess %s for this collaboration", c.Name)
			a.Fit = c.Reason
			coverage := "Licensed territories require verification"
			if len(s.Markets) > 0 {
				coverage = "Library coverage: " + strings.Join(ipEnglishList(s.Markets), ", ") + "; licensed territories require verification"
			}
			a.Basis = []ipRecommendationBasis{{"Request fit", c.Reason}, {"IP type", ipEnglishText(recommendationString(c.Resource["ipType"]))}, {"Regional conditions", coverage}}
			a.EntryPoints = c.Scenarios
			a.Pending = []string{"Territorial rights and permitted use of assets", "Pricing, availability and rights-holder resources"}
			c.Assessment = &a
		}
	} else {
		if caution := recommendationRightsCaution(c); caution != "" {
			c.Advice = strings.ReplaceAll(c.Advice, caution, rights)
		}
		c.Advice = strings.ReplaceAll(c.Advice, " 外部背景：", " External context: ")
		if c.Assessment != nil {
			a := *c.Assessment
			a.Pending = ipEnglishList(a.Pending)
			c.Assessment = &a
		}
	}
	return c
}

// Normalize only the parser's working copy. Original messages, names and stored criteria stay unchanged.
func normalizeIPIntentTerms(text string) string {
	type phrase struct{ pattern, replacement string }
	for _, p := range []phrase{
		{`\b(?:product co-branding|co-branding|co branding|product licensing|product collaboration)\b`, "产品联名"},
		{`\b(?:event sponsorship|tournament sponsorship|sponsorship)\b`, "赛事赞助"},
		{`\b(?:content marketing|content collaboration|content campaign|content distribution)\b`, "内容传播"},
		{`\b(?:art and design|art & design|art licensing|arts?|designers?|design)\b`, "艺术设计"},
		{`\b(?:e-sports?|esports?)\b`, "电竞"}, {`\b(?:video games?|gaming|games?)\b`, "游戏"},
		{`\b(?:animations?|anime|cartoons?)\b`, "动画"}, {`\b(?:films?|movies?|television)\b`, "影视"},
		{`\b(?:sports?)\b`, "体育"}, {`\b(?:teams?|clubs?)\b`, "战队"}, {`\b(?:tournaments?|leagues?)\b`, "赛事"},
		{`\b(?:players?|athletes?)\b`, "选手"}, {`\b(?:fashion|streetwear)\b`, "潮牌"},
		{`\b(?:explore first|start exploring|just explore|explore broadly|explore for now)\b`, "先探索"},
		{`\b(?:continue researching|continue research|research further|keep researching|continue checking|follow up on|look further)\b`, "继续深入"},
		{`\b(?:not interested in|do not include|don't include|exclude|avoid|without|no)\s+`, "排除"},
		{`\b(?:only)\s+`, "只看"}, {`\b(?:prioritize|prefer|focus on|look for|find)\s+`, "优先"},
	} {
		text = regexp.MustCompile("(?i)"+p.pattern).ReplaceAllString(text, p.replacement)
	}
	markets := make([]string, 0, len(researchMarketNames))
	for market := range researchMarketNames {
		markets = append(markets, market)
	}
	sort.Slice(markets, func(i, j int) bool {
		return len(researchMarketNames[markets[i]]) > len(researchMarketNames[markets[j]])
	})
	for _, market := range markets {
		text = regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(researchMarketNames[market])+`\b`).ReplaceAllString(text, market)
	}
	return text
}
