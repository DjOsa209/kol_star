package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// excelContentAliases intentionally uses an ordered list. Several common header
// words (for example "Channel" and "Type") are broad, so a map would make the
// matching result depend on Go's random map iteration order.
var excelContentAliases = []struct {
	field   string
	aliases []string
}{
	{"influencer", []string{"姓名", "达人", "博主", "账号", "作者", "主播", "influencer", "creator", "kol", "media/kol", "publication", "outlet", "outlet name", "media name"}},
	{"category", []string{"领域", "分类", "类别", "垂类", "行业", "类型", "category", "industry", "vertical", "niche", "content type", "source type", "type"}},
	{"platform", []string{"平台", "渠道", "platform", "channel", "social platform", "social media"}},
	{"country", []string{"国家", "地区", "国家地区", "region", "country", "location", "market"}},
	{"followerNumber", []string{"粉丝数", "粉丝量", "粉丝", "follower number", "followers", "follower", "fans", "subscribers", "audience size", "uvm", "muv"}},
	{"releaseDate", []string{"发布日期", "发布时间", "发布日", "release date", "publish date", "published date", "publication date", "posted date", "post date", "live date"}},
	{"deliverableLinks", []string{"发布链接", "内容链接", "作品链接", "原文链接", "链接", "网址", "deliverable links", "deliverable link", "published link", "published url", "content link", "content url", "post link", "post url", "video url", "article url", "link", "url"}},
	{"views", []string{"播放量", "浏览量", "阅读量", "曝光量", "views", "view count", "video views", "impressions", "reach"}},
	{"engagementCount", []string{"转赞藏数", "互动量", "互动数", "点赞收藏转发", "likes+fav+share", "likes fav share", "total engagement", "engagement count", "engagements", "engagement", "interactions"}},
	{"commentsCount", []string{"评论数", "评论量", "comments", "comment count"}},
}

var excelNumericFields = map[string]bool{
	"followerNumber":  true,
	"views":           true,
	"engagementCount": true,
	"commentsCount":   true,
}

func (a *app) previewProjectExcelImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusOK, 10001, "请选择 Excel 文件")
		return
	}
	var fileName string
	var content []byte
	for {
		part, nextErr := reader.NextPart()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			writeError(w, http.StatusOK, 10001, "Excel 上传内容读取失败")
			return
		}
		if part.FormName() != "file" {
			part.Close()
			continue
		}
		fileName = part.FileName()
		content, err = io.ReadAll(io.LimitReader(part, (100<<20)+1))
		part.Close()
		if err != nil {
			writeError(w, http.StatusOK, 10001, "Excel 文件不能超过 100MB")
			return
		}
		if len(content) > 100<<20 {
			writeError(w, http.StatusOK, 10001, "Excel 文件不能超过 100MB")
			return
		}
		break
	}
	if len(content) == 0 {
		writeError(w, http.StatusOK, 10001, "请选择 Excel 文件")
		return
	}
	book, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		writeError(w, http.StatusOK, 10001, "Excel 文件无法读取")
		return
	}
	defer book.Close()
	sheets := make([]map[string]any, 0)
	for _, name := range book.GetSheetList() {
		rows, err := parseExcelContentSheet(book, name)
		if err != nil {
			writeError(w, http.StatusOK, 10001, fmt.Sprintf("Sheet %s 解析失败", name))
			return
		}
		sheets = append(sheets, map[string]any{"name": name, "rows": rows})
	}
	writeOK(w, map[string]any{"fileName": fileName, "sheets": sheets})
}

func parseExcelContentSheet(book *excelize.File, sheet string) ([]map[string]any, error) {
	grid, err := book.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	mergedValues, err := mergedCellValues(book, sheet)
	if err != nil {
		return nil, err
	}
	headers := []string(nil)
	previous := map[string]any{}
	result := make([]map[string]any, 0)
	for rowIndex, values := range grid {
		if excelHeaderRow(values) {
			headers = values
			previous = map[string]any{}
			continue
		}
		if len(headers) == 0 || allBlank(values) {
			continue
		}
		row := map[string]any{}
		for column, header := range headers {
			if strings.TrimSpace(header) == "" {
				continue
			}
			key := excelFieldForHeader(header)
			if key == "" {
				continue
			}
			cell, _ := excelize.CoordinatesToCellName(column+1, rowIndex+1)
			value := ""
			if column < len(values) {
				value = strings.TrimSpace(values[column])
			}
			if value == "" {
				value = mergedValues[cell]
			}
			if key == "deliverableLinks" {
				if ok, link, _ := book.GetCellHyperLink(sheet, cell); ok && strings.TrimSpace(link) != "" {
					value = strings.ReplaceAll(link, "&amp;", "&")
				}
			}
			if excelNumericFields[key] {
				// An empty metric cell is deliberately left absent. This lets a
				// repeated creator row inherit its profile follower count, while an
				// explicit NaN still normalizes to zero below.
				if value != "" {
					row[key] = excelNumberValue(value)
				}
			} else {
				row[key] = value
			}
		}
		contentLinks := mappedContentLinks(
			book, sheet, rowIndex, headers, values, mergedValues,
		)
		if len(contentLinks) == 0 {
			contentLinks = fallbackContentLinks(
				book, sheet, rowIndex, headers, values, mergedValues,
			)
		}
		// A visible cell value is not necessarily a usable URL. Keep only an
		// actual HTTP(S) link so a malformed field becomes empty during preview
		// instead of failing the database write later.
		row["deliverableLinks"] = ""
		if len(contentLinks) > 0 {
			row["deliverableLinks"] = contentLinks[0]
		}
		row["releaseDate"] = normalizeImportedDate(excelCellString(row["releaseDate"]))
		if excelCellString(row["influencer"]) == "" && excelCellString(row["deliverableLinks"]) == "" && excelCellString(row["platform"]) == "" && excelCellString(row["category"]) == "" {
			continue // 汇总行
		}
		inheritedInfluencer := excelCellString(row["influencer"]) == ""
		if inheritedInfluencer {
			row["influencer"] = previous["influencer"]
		}
		if excelCellString(row["category"]) == "" {
			row["category"] = previous["category"]
		}
		if excelCellString(row["platform"]) == "" {
			row["platform"] = previous["platform"]
		}
		if inheritedInfluencer {
			if _, exists := row["followerNumber"]; !exists {
				row["followerNumber"] = previous["followerNumber"]
			}
			if excelCellString(row["country"]) == "" {
				row["country"] = previous["country"]
			}
		}
		if excelCellString(row["influencer"]) == "" {
			continue
		}
		if excelCellString(row["platform"]) == "" && excelCellString(row["deliverableLinks"]) != "" {
			row["platform"] = platformFromLink(excelCellString(row["deliverableLinks"]))
		}
		row["rowNo"] = rowIndex + 1
		row["sourceSheet"] = sheet
		row["resourceType"] = "KOL"
		identityHeader := excelHeaderNorm(headerText(headers, "influencer"))
		if strings.EqualFold(excelCellString(row["platform"]), "website") || strings.Contains(identityHeader, "publication") || strings.Contains(identityHeader, "mediaoutlet") {
			row["resourceType"] = "媒体"
		}
		row["mediaOutlet"] = ""
		if row["resourceType"] == "媒体" {
			row["mediaOutlet"] = excelCellString(row["influencer"])
			if excelCellString(row["platform"]) == "" {
				row["platform"] = "Website"
			}
		}
		for field := range excelNumericFields {
			if _, exists := row[field]; !exists {
				row[field] = float64(0)
			}
		}
		if len(contentLinks) == 0 {
			result = append(result, row)
		} else {
			// A tracker cell can contain several published URLs. Preserve every
			// URL as a separate imported content record instead of silently
			// keeping only the first one.
			for _, contentLink := range contentLinks {
				contentRow := make(map[string]any, len(row))
				for key, value := range row {
					contentRow[key] = value
				}
				contentRow["deliverableLinks"] = contentLink
				result = append(result, contentRow)
			}
		}
		previous = row
	}
	return result, nil
}

func mappedContentLinks(book *excelize.File, sheet string, rowIndex int, headers, row []string, mergedValues map[string]string) []string {
	links := make([]string, 0)
	for column, header := range headers {
		if excelFieldForHeader(header) != "deliverableLinks" {
			continue
		}
		links = append(links, excelCellLinks(book, sheet, rowIndex, column, row, mergedValues)...)
	}
	return uniqueExcelLinks(links)
}

func fallbackContentLinks(book *excelize.File, sheet string, rowIndex int, headers, row []string, mergedValues map[string]string) []string {
	links := make([]string, 0)
	columnCount := len(row)
	if len(headers) > columnCount {
		columnCount = len(headers)
	}
	for column := 0; column < columnCount; column++ {
		header := ""
		if column < len(headers) {
			header = headers[column]
		}
		if isProfileLinkHeader(header) {
			continue
		}
		links = append(links, excelCellLinks(book, sheet, rowIndex, column, row, mergedValues)...)
	}
	return uniqueExcelLinks(links)
}

func excelCellLinks(book *excelize.File, sheet string, rowIndex, column int, row []string, mergedValues map[string]string) []string {
	cell, _ := excelize.CoordinatesToCellName(column+1, rowIndex+1)
	links := make([]string, 0)
	if ok, link, _ := book.GetCellHyperLink(sheet, cell); ok {
		link = strings.ReplaceAll(link, "&amp;", "&")
		links = append(links, httpExcelURLs(link)...)
	}
	value := ""
	if column < len(row) {
		value = row[column]
	}
	if strings.TrimSpace(value) == "" {
		value = mergedValues[cell]
	}
	links = append(links, httpExcelURLs(value)...)
	return uniqueExcelLinks(links)
}

func firstHTTPExcelURL(value string) string {
	links := httpExcelURLs(value)
	if len(links) > 0 {
		return links[0]
	}
	return ""
}

func httpExcelURLs(value string) []string {
	links := make([]string, 0)
	for _, field := range strings.Fields(strings.ReplaceAll(value, "&amp;", "&")) {
		remaining := field
		for {
			index := strings.Index(strings.ToLower(remaining), "http")
			if index < 0 {
				break
			}
			candidate := strings.Trim(remaining[index:], "，,;；。.!?)）]\"")
			next := strings.Index(strings.ToLower(candidate[4:]), "http")
			if next >= 0 {
				candidate = candidate[:next+4]
			}
			if isHTTPExcelURL(candidate) {
				links = append(links, candidate)
			}
			if len(candidate) >= len(remaining[index:]) {
				break
			}
			remaining = remaining[index+len(candidate):]
		}
	}
	return uniqueExcelLinks(links)
}

func uniqueExcelLinks(links []string) []string {
	seen := make(map[string]bool, len(links))
	result := make([]string, 0, len(links))
	for _, link := range links {
		link = strings.TrimSpace(link)
		key := strings.ToLower(link)
		if link == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, link)
	}
	return result
}

func isProfileLinkHeader(header string) bool {
	normalized := excelHeaderNorm(header)
	for _, value := range []string{"profile", "homepage", "channel", "account", "website", "domain", "账号", "主页"} {
		if strings.Contains(normalized, value) {
			return true
		}
	}
	return false
}

func mergedCellValues(book *excelize.File, sheet string) (map[string]string, error) {
	mergedCells, err := book.GetMergeCells(sheet)
	if err != nil {
		return nil, err
	}
	values := make(map[string]string)
	for index := range mergedCells {
		startColumn, startRow, err := excelize.CellNameToCoordinates(mergedCells[index].GetStartAxis())
		if err != nil {
			return nil, err
		}
		endColumn, endRow, err := excelize.CellNameToCoordinates(mergedCells[index].GetEndAxis())
		if err != nil {
			return nil, err
		}
		value := strings.TrimSpace(mergedCells[index].GetCellValue())
		if value == "" {
			continue
		}
		for row := startRow; row <= endRow; row++ {
			for column := startColumn; column <= endColumn; column++ {
				cell, _ := excelize.CoordinatesToCellName(column, row)
				values[cell] = value
			}
		}
	}
	return values, nil
}

func excelHeaderRow(values []string) bool {
	matches := 0
	hasLinkHeader := false
	hasIdentityHeader := false
	for _, value := range values {
		value = strings.TrimSpace(value)
		// Content rows almost always contain the published URL. A URL must never
		// be allowed to reset the active header mapping.
		if isHTTPExcelURL(value) {
			return false
		}
		field := excelFieldForHeader(value)
		if field != "" {
			matches++
			hasLinkHeader = hasLinkHeader || field == "deliverableLinks"
			hasIdentityHeader = hasIdentityHeader || field == "influencer"
		}
	}
	// A profile sheet usually has an identity column but no published-link
	// column. It still belongs to the project, while content links are handled
	// separately when they are present.
	return matches >= 2 && (hasLinkHeader || hasIdentityHeader)
}

func isHTTPExcelURL(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://")
}

func excelFieldForHeader(header string) string {
	for _, definition := range excelContentAliases {
		for _, alias := range definition.aliases {
			if excelHeaderMatch(header, alias) {
				return definition.field
			}
		}
	}
	return ""
}
func excelHeaderMatch(header, alias string) bool {
	h, a := excelHeaderNorm(header), excelHeaderNorm(alias)
	if a == "url" || a == "link" {
		return h == a
	}
	return h == a || (len([]rune(a)) >= 3 && strings.Contains(h, a))
}
func excelHeaderNorm(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (r >= 0x4e00 && r <= 0x9fff) {
			return r
		}
		return -1
	}, value)
}
func allBlank(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}
func headerText(headers []string, target string) string {
	for _, header := range headers {
		if excelFieldForHeader(header) == target {
			return strings.TrimSpace(header)
		}
	}
	return ""
}

func excelCellString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func excelNumberValue(value string) float64 {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == "-" || value == "--" || value == "n/a" || value == "na" || value == "nan" {
		return 0
	}
	multiplier := float64(1)
	switch {
	case strings.HasSuffix(value, "k"):
		multiplier, value = 1000, strings.TrimSuffix(value, "k")
	case strings.HasSuffix(value, "m"):
		multiplier, value = 1000000, strings.TrimSuffix(value, "m")
	case strings.HasSuffix(value, "万"):
		multiplier, value = 10000, strings.TrimSuffix(value, "万")
	}
	value = strings.NewReplacer(",", "", "，", "", " ", "", "+", "").Replace(value)
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || number < 0 {
		return 0
	}
	return number * multiplier
}
func platformFromLink(link string) string {
	link = strings.ToLower(link)
	switch {
	case strings.Contains(link, "youtube") || strings.Contains(link, "youtu.be"):
		return "YouTube"
	case strings.Contains(link, "tiktok"):
		return "TikTok"
	case strings.Contains(link, "instagram"):
		return "Instagram"
	case strings.Contains(link, "x.com/") || strings.Contains(link, "twitter.com/"):
		return "X"
	default:
		return "Website"
	}
}
