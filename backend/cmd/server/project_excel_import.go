package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

const standardProjectCostNumberFormat = `"$"#,##0.00;-"$"#,##0.00`

// excelContentAliases is deliberately strict: the standard workbook is the
// only supported import contract. Legacy Chinese/English aliases must not be
// accepted silently because that would make a malformed template look valid.
var excelContentAliases = []struct {
	field   string
	aliases []string
}{
	{"resourceName", []string{"Name"}},
	{"influencer", []string{"collaboratorName"}},
	{"resourceType", []string{"resourceType"}},
	{"category", []string{"category"}},
	{"country", []string{"market"}},
	{"followerNumber", []string{"audienceSize"}},
	{"collaboratorTier", []string{"collaboratorTier"}},
	{"platform", []string{"platform"}},
	{"cooperationType", []string{"collaborationType"}},
	{"deliverableLinks", []string{"contentUrl"}},
	{"contentType", []string{"contentType"}},
	{"quoteAmount", []string{"cost"}},
	{"views", []string{"views"}},
	{"engagementCount", []string{"engagement"}},
	{"primaryContact", []string{"primaryContact"}},
	{"owner", []string{"owner"}},
	{"vendor", []string{"vendor"}},
	{"notes", []string{"notes"}},
	{"cpm", []string{"CPM"}},
}

var standardProjectImportHeaders = []string{
	"标准字段", "Name", "collaboratorName", "resourceType", "category", "market", "audienceSize",
	"collaboratorTier", "platform", "collaborationType", "contentUrl", "contentType", "cost", "views",
	"engagement", "primaryContact", "owner", "vendor", "notes", "CPM",
}

var standardProjectImportLabels = []string{
	"", "名称", "合作方", "类型", "领域", "市场", "粉丝数/访问量", "合作方层级", "平台", "合作类型",
	"内容链接", "内容类型", "合作费用", "曝光量/播放量", "互动量", "联系方式", "对接人", "供应商", "备注", "",
}

var standardProjectImportScopes = []string{
	"填写范畴",
	"媒体名称\n达人名称",
	"媒体官网链接\n创作者账号链接",
	"KOL\n媒体\n艺术家",
	"科技\n生活方式\n商业\n设计\n游戏\n摄影\n体育\n娱乐\n汽车\n财经\n教育\n大众媒体",
	"国家\n市场\n地区\n区域",
	"系统自动回填：\nKOL：粉丝数、订阅数；\n媒体：月独立访客（UMV）",
	"系统自动回填：\n头部\n腰部\n尾部",
	"网站\n播客\n电视\n报刊\nYouTube\nTikTok\nInstagram\nFacebook\nX\nLinkedIn\nReddit",
	"付费合作\n产品置换\n联盟合作\n活动合作\n采访合作",
	"文章链接\n视频链接\n帖子链接\n内容链接\n发布链接",
	"生活记录类\n娱乐搞笑类\n兴趣圈层类\n消费种草类\n商业/品牌类\n新闻资讯类\n动画/创意类\n短剧类",
	"paid费用填写：实际支付金额\nseeding或不产生费用则填写：/",
	"系统通过 contentUrl 自动回填播放量、曝光量或阅读量",
	"系统通过 contentUrl 自动回填总互动量（点赞、评论、分享、收藏）",
	"邮箱 / WhatsApp / Telegram",
	"In-house项目对接人",
	"项目供应商",
	"",
	"",
}

var standardProjectImportRules = []string{
	"填写规范",
	"填写对应合作方名称",
	"1. 填写合作方对应网址\n-媒体填写官网链接\n-KOL填写达人对应平台主页\n\n2.一个资源对应一条URL，填写完整https链接",
	"使用平台预设分类表述，不允许自由填写",
	"使用平台预设分类表述，不允许新增自由分类",
	"使用标准国家名称，不使用简称",
	"无需填写。系统根据合作方主页链接自动抓取最新数据。",
	"无需填写。系统根据 audienceSize 自动归类：头部 > 100万；腰部 10万-100万；尾部 < 10万。",
	"1.使用平台标准名称，而非缩写\n2.媒体网站资源对应平台填写Website",
	"使用系统预设选项",
	"一个内容对应一条URL，填写完整https链接",
	"生活记录类：日常vlog/旅行vlog/职场日常\n娱乐搞笑类：搞笑情景剧/恶搞整蛊/挑战类\n兴趣圈层类：科技数码测评/游戏实况/时尚穿搭/音乐原创/舞蹈原创\n消费种草类：开箱体验/购物分享/年度榜单\n商业/品牌类：品牌科普/品牌活动记录\n新闻资讯类：热点解读/社会纪实/科技前沿/本地资讯\n动画/创意类：原创动画/创意实验\n短剧类：都市情感短剧/家庭伦理剧/悬疑推理剧",
	"填写实际支付美元金额（数字），币种统一使用Currency字段，不在金额中填写货币符号",
	"无需填写。系统通过 contentUrl 解析；暂不支持的平台保留为空并记录同步提示。",
	"无需填写。系统通过 contentUrl 解析；暂不支持的平台保留为空并记录同步提示。",
	"可自由编辑",
	"可自由编辑",
	"可自由编辑",
	"可自由编辑\n媒体/达人合作方式、配合程度等备注",
	"无需填写。系统按 CPM = Cost / views × 1000 自动计算。",
}

var excelNumericFields = map[string]bool{
	"followerNumber":  true,
	"views":           true,
	"engagementCount": true,
	"quoteAmount":     true,
	"cpm":             true,
}

var defaultStandardProjectImportOptions = map[string][]string{
	"resourceType":     {"KOL", "媒体", "艺术家"},
	"category":         {"科技", "生活方式", "商业", "设计", "游戏", "摄影", "体育", "娱乐", "汽车", "财经", "教育", "大众媒体"},
	"collaboratorTier": {"头部", "腰部", "尾部"},
	"platform":         {"Website", "播客", "电视", "报刊", "YouTube", "TikTok", "Instagram", "Facebook", "X", "LinkedIn", "Reddit"},
	"cooperationType":  {"付费合作", "产品置换", "联盟合作", "活动合作", "采访合作"},
	"contentType":      {"生活记录类", "娱乐搞笑类", "兴趣圈层类", "消费种草类", "商业/品牌类", "新闻资讯类", "动画/创意类", "短剧类"},
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
	options, err := a.standardImportOptions(r.Context())
	if err != nil {
		writeDBError(w, err)
		return
	}
	sheets := make([]map[string]any, 0)
	for _, name := range book.GetSheetList() {
		rows, err := parseExcelContentSheetWithOptions(book, name, options)
		if err != nil {
			writeError(w, http.StatusOK, 10001, fmt.Sprintf("Sheet %s：%v", name, err))
			return
		}
		sheets = append(sheets, map[string]any{"name": name, "rows": rows})
	}
	writeOK(w, map[string]any{
		"fileName":    fileName,
		"projectName": projectNameFromUploadFileName(fileName),
		"sheets":      sheets,
	})
}

func projectNameFromUploadFileName(fileName string) string {
	base := filepath.Base(strings.TrimSpace(fileName))
	extension := filepath.Ext(base)
	return strings.TrimSpace(strings.TrimSuffix(base, extension))
}

func (a *app) downloadProjectExcelImportTemplate(w http.ResponseWriter, r *http.Request) {
	options, err := a.standardImportOptions(r.Context())
	if err != nil {
		writeDBError(w, err)
		return
	}
	book, err := buildStandardProjectImportTemplateWithOptions(options)
	if err != nil {
		writeError(w, http.StatusInternalServerError, 10006, "标准模板生成失败")
		return
	}
	defer book.Close()
	buffer, err := book.WriteToBuffer()
	if err != nil {
		writeError(w, http.StatusInternalServerError, 10006, "标准模板生成失败")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="XMP_Standard_Project_Import.xlsx"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buffer.Bytes())
}

func buildStandardProjectImportTemplate() (*excelize.File, error) {
	return buildStandardProjectImportTemplateWithOptions(defaultStandardProjectImportOptions)
}

func buildStandardProjectImportTemplateWithOptions(options map[string][]string) (*excelize.File, error) {
	book := excelize.NewFile()
	sheet := "标准模板"
	defaultSheet := book.GetSheetName(0)
	if err := book.SetSheetName(defaultSheet, sheet); err != nil {
		return nil, err
	}
	headerRow := make([]any, len(standardProjectImportHeaders))
	labelRow := make([]any, len(standardProjectImportLabels))
	scopeRow := make([]any, len(standardProjectImportScopes))
	ruleRow := make([]any, len(standardProjectImportRules))
	for index := range standardProjectImportHeaders {
		headerRow[index] = standardProjectImportHeaders[index]
		if standardProjectImportLabels[index] != "" {
			labelRow[index] = standardProjectImportLabels[index]
		}
		if standardProjectImportScopes[index] != "" {
			scopeRow[index] = standardProjectImportScopes[index]
		}
		if standardProjectImportRules[index] != "" {
			ruleRow[index] = standardProjectImportRules[index]
		}
	}
	if err := book.SetSheetRow(sheet, "A1", &headerRow); err != nil {
		return nil, err
	}
	if err := book.SetSheetRow(sheet, "A2", &labelRow); err != nil {
		return nil, err
	}
	if err := book.SetSheetRow(sheet, "A3", &scopeRow); err != nil {
		return nil, err
	}
	if err := book.SetSheetRow(sheet, "A4", &ruleRow); err != nil {
		return nil, err
	}
	if err := book.MergeCell(sheet, "A1", "A2"); err != nil {
		return nil, err
	}
	if err := book.MergeCell(sheet, "T1", "T2"); err != nil {
		return nil, err
	}

	newHeaderStyle := func(fill string, bold bool) (int, error) {
		return book.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: bold, Color: "#000000", Family: "Arial", Size: 10},
			Fill:      excelize.Fill{Type: "pattern", Color: []string{fill}, Pattern: 1},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
			Border: []excelize.Border{
				{Type: "left", Color: "#D0D0D0", Style: 1}, {Type: "right", Color: "#D0D0D0", Style: 1},
				{Type: "top", Color: "#D0D0D0", Style: 1}, {Type: "bottom", Color: "#D0D0D0", Style: 1},
			},
			Protection: &excelize.Protection{Locked: true},
		})
	}
	blue, err := newHeaderStyle("#BACEFD", true)
	if err != nil {
		return nil, err
	}
	yellow, err := newHeaderStyle("#FAF1D1", true)
	if err != nil {
		return nil, err
	}
	gray, err := newHeaderStyle("#DEE0E3", true)
	if err != nil {
		return nil, err
	}
	green, err := newHeaderStyle("#EEF6C6", true)
	if err != nil {
		return nil, err
	}
	if err := book.SetCellStyle(sheet, "A1", "A1", blue); err != nil {
		return nil, err
	}
	for _, group := range []struct {
		start string
		end   string
		style int
	}{
		{"B1", "F2", yellow}, {"G1", "H2", gray}, {"I1", "M2", yellow},
		{"N1", "O2", gray}, {"P1", "S2", green}, {"T1", "T1", gray},
	} {
		if err := book.SetCellStyle(sheet, group.start, group.end, group.style); err != nil {
			return nil, err
		}
	}
	editableStyle, err := book.NewStyle(&excelize.Style{Protection: &excelize.Protection{Locked: false}})
	if err != nil {
		return nil, err
	}
	if err := book.SetColStyle(sheet, "B:S", editableStyle); err != nil {
		return nil, err
	}
	if err := book.SetCellStyle(sheet, "B5", "S2000", editableStyle); err != nil {
		return nil, err
	}
	costStyle, err := book.NewStyle(&excelize.Style{
		CustomNumFmt: stringPointer(standardProjectCostNumberFormat),
		Protection:   &excelize.Protection{Locked: false},
	})
	if err != nil {
		return nil, err
	}
	if err := book.SetCellStyle(sheet, "M5", "M2000", costStyle); err != nil {
		return nil, err
	}
	calculatedStyle, err := book.NewStyle(&excelize.Style{
		Fill:       excelize.Fill{Type: "pattern", Color: []string{"#F3F4F6"}, Pattern: 1},
		Protection: &excelize.Protection{Locked: true},
	})
	if err != nil {
		return nil, err
	}
	// audienceSize、collaboratorTier、views、engagement 和 CPM 都由平台逻辑
	// 自动回填。即使用户解除工作表保护，解析器也不会采纳这些列的值。
	for _, column := range []string{"G", "H", "N", "O", "T"} {
		if err := book.SetCellStyle(sheet, column+"5", column+"2000", calculatedStyle); err != nil {
			return nil, err
		}
	}
	for _, definition := range []struct {
		column string
		field  string
	}{
		{"D", "resourceType"}, {"E", "category"}, {"I", "platform"}, {"J", "cooperationType"}, {"L", "contentType"},
	} {
		validation := excelize.NewDataValidation(true)
		validation.Sqref = definition.column + "5:" + definition.column + "2000"
		if err := validation.SetDropList(options[definition.field]); err != nil {
			return nil, err
		}
		validation.SetError(excelize.DataValidationErrorStyleStop, "非标准选项", "请从平台预设选项中选择")
		if err := book.AddDataValidation(sheet, validation); err != nil {
			return nil, err
		}
	}
	// Column styles apply to the whole column, so lock and restyle the two fixed
	// header rows after making the data-entry columns editable.
	for _, group := range []struct {
		start string
		end   string
		style int
	}{
		{"B1", "F2", yellow}, {"G1", "H2", gray}, {"I1", "M2", yellow},
		{"N1", "O2", gray}, {"P1", "S2", green},
	} {
		if err := book.SetCellStyle(sheet, group.start, group.end, group.style); err != nil {
			return nil, err
		}
	}
	instructionStyle, err := book.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "#000000", Family: "Arial", Size: 9},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "#D0D0D0", Style: 1}, {Type: "right", Color: "#D0D0D0", Style: 1},
			{Type: "top", Color: "#D0D0D0", Style: 1}, {Type: "bottom", Color: "#D0D0D0", Style: 1},
		},
		Protection: &excelize.Protection{Locked: true},
	})
	if err != nil {
		return nil, err
	}
	if err := book.SetCellStyle(sheet, "B3", "T4", instructionStyle); err != nil {
		return nil, err
	}
	if err := book.SetCellStyle(sheet, "A3", "A4", blue); err != nil {
		return nil, err
	}
	widths := []float64{12, 25, 25, 16, 16, 14, 18, 18, 16, 20, 32, 32, 14, 18, 16, 24, 16, 16, 24, 12}
	for index, width := range widths {
		column, _ := excelize.ColumnNumberToName(index + 1)
		if err := book.SetColWidth(sheet, column, column, width); err != nil {
			return nil, err
		}
	}
	_ = book.SetRowHeight(sheet, 1, 26)
	_ = book.SetRowHeight(sheet, 2, 28)
	_ = book.SetRowHeight(sheet, 3, 148)
	_ = book.SetRowHeight(sheet, 4, 148)
	if err := book.SetPanes(sheet, &excelize.Panes{Freeze: true, Split: false, YSplit: 4, TopLeftCell: "A5", ActivePane: "bottomLeft"}); err != nil {
		return nil, err
	}
	if err := book.ProtectSheet(sheet, &excelize.SheetProtectionOptions{
		Password: "xmp-standard-template", SelectLockedCells: false, SelectUnlockedCells: true,
	}); err != nil {
		return nil, err
	}
	if err := book.ProtectWorkbook(&excelize.WorkbookProtectionOptions{Password: "xmp-standard-template", LockStructure: true}); err != nil {
		return nil, err
	}
	return book, nil
}

func buildStandardProjectExportWorkbook(options map[string][]string, rows []map[string]any) (*excelize.File, error) {
	book, err := buildStandardProjectImportTemplateWithOptions(options)
	if err != nil {
		return nil, err
	}
	if err := populateStandardProjectExportRows(book, rows); err != nil {
		book.Close()
		return nil, err
	}
	return book, nil
}

func populateStandardProjectExportRows(book *excelize.File, rows []map[string]any) error {
	const sheet = "标准模板"
	editableNumberStyle, err := book.NewStyle(&excelize.Style{
		CustomNumFmt: stringPointer(standardProjectCostNumberFormat),
		Protection:   &excelize.Protection{Locked: false},
	})
	if err != nil {
		return err
	}
	calculatedIntegerStyle, err := book.NewStyle(&excelize.Style{
		CustomNumFmt: stringPointer("#,##0"),
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#F3F4F6"}, Pattern: 1},
		Protection:   &excelize.Protection{Locked: true},
	})
	if err != nil {
		return err
	}
	calculatedDecimalStyle, err := book.NewStyle(&excelize.Style{
		CustomNumFmt: stringPointer("#,##0.00"),
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#F3F4F6"}, Pattern: 1},
		Protection:   &excelize.Protection{Locked: true},
	})
	if err != nil {
		return err
	}

	for index, row := range rows {
		rowNumber := index + 5
		profileURL := firstHTTPExcelURL(stringValue(row["profileUrl"]))
		contentValue := strings.TrimSpace(stringValue(row["contentUrl"]))
		values := []any{
			nil,
			excelTextOrNil(firstNonEmpty(stringValue(row["resourceName"]), profileURL)),
			excelTextOrNil(profileURL), excelTextOrNil(row["resourceType"]), excelTextOrNil(row["category"]), excelTextOrNil(row["market"]), row["audienceSize"],
			excelTextOrNil(row["collaboratorTier"]), excelTextOrNil(row["platform"]), excelTextOrNil(row["cooperationType"]), excelTextOrNil(contentValue), excelTextOrNil(row["contentType"]),
			row["cost"], row["views"], row["engagement"], excelTextOrNil(row["primaryContact"]),
			excelTextOrNil(row["owner"]), excelTextOrNil(row["vendor"]), excelTextOrNil(row["notes"]), row["cpm"],
		}
		startCell, _ := excelize.CoordinatesToCellName(1, rowNumber)
		if err := book.SetSheetRow(sheet, startCell, &values); err != nil {
			return err
		}
		if profileURL != "" {
			if err := book.SetCellHyperLink(sheet, fmt.Sprintf("C%d", rowNumber), profileURL, "External"); err != nil {
				return err
			}
		}
		if contentURL := firstHTTPExcelURL(contentValue); contentURL != "" {
			if err := book.SetCellHyperLink(sheet, fmt.Sprintf("K%d", rowNumber), contentURL, "External"); err != nil {
				return err
			}
		}
		if err := book.SetCellStyle(sheet, fmt.Sprintf("M%d", rowNumber), fmt.Sprintf("M%d", rowNumber), editableNumberStyle); err != nil {
			return err
		}
		for _, column := range []string{"G", "N", "O"} {
			if err := book.SetCellStyle(sheet, fmt.Sprintf("%s%d", column, rowNumber), fmt.Sprintf("%s%d", column, rowNumber), calculatedIntegerStyle); err != nil {
				return err
			}
		}
		if err := book.SetCellStyle(sheet, fmt.Sprintf("T%d", rowNumber), fmt.Sprintf("T%d", rowNumber), calculatedDecimalStyle); err != nil {
			return err
		}
		_ = book.SetRowHeight(sheet, rowNumber, 24)
	}
	return nil
}

func stringPointer(value string) *string {
	return &value
}

func excelTextOrNil(value any) any {
	text := strings.TrimSpace(stringValue(value))
	if text == "" {
		return nil
	}
	return text
}

func parseExcelContentSheet(book *excelize.File, sheet string) ([]map[string]any, error) {
	return parseExcelContentSheetWithOptions(book, sheet, defaultStandardProjectImportOptions)
}

func parseExcelContentSheetWithOptions(book *excelize.File, sheet string, options map[string][]string) ([]map[string]any, error) {
	grid, err := book.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	mergedValues, err := mergedCellValues(book, sheet)
	if err != nil {
		return nil, err
	}
	if len(grid) < 4 || !standardImportRowMatches(grid[0], standardProjectImportHeaders) {
		return nil, fmt.Errorf("第1行必须是锁定的标准字段表头")
	}
	if !standardImportRowMatches(grid[1], standardProjectImportLabels) {
		return nil, fmt.Errorf("第2行中文表头与标准模板不一致")
	}
	if !standardImportInstructionRowMatches(grid[2], standardProjectImportScopes[0]) {
		return nil, fmt.Errorf("第3行必须保留填写范畴说明")
	}
	if !standardImportInstructionRowMatches(grid[3], standardProjectImportRules[0]) {
		return nil, fmt.Errorf("第4行必须保留填写规范说明")
	}
	headers := grid[0]
	previous := map[string]any{}
	result := make([]map[string]any, 0)
	for rowIndex := 4; rowIndex < len(grid); rowIndex++ {
		values := grid[rowIndex]
		if allBlank(values) {
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
			if key == "deliverableLinks" || key == "influencer" {
				visibleLinks := httpExcelURLs(value)
				if len(visibleLinks) > 0 {
					value = visibleLinks[0]
				} else if ok, link, _ := book.GetCellHyperLink(sheet, cell); ok && strings.TrimSpace(link) != "" {
					value = strings.ReplaceAll(link, "&amp;", "&")
				}
			}
			if excelNumericFields[key] {
				// An empty metric cell is deliberately left absent. This lets a
				// repeated creator row inherit its profile follower count, while an
				// explicit NaN still normalizes to zero below.
				numericValue := value
				if rawValue, rawErr := book.GetCellValue(
					sheet,
					cell,
					excelize.Options{RawCellValue: true},
				); rawErr == nil && strings.TrimSpace(rawValue) != "" {
					// GetRows returns the formatted display value (for example
					// "$2,500.00"). Prefer the underlying numeric value so Excel
					// currency and accounting formats do not erase the cost.
					numericValue = rawValue
				}
				if numericValue != "" {
					row[key] = excelNumberValue(numericValue)
				}
			} else {
				row[key] = value
			}
		}
		contentLinks := mappedContentLinks(
			book, sheet, rowIndex, headers, values, mergedValues,
		)
		// A visible cell value is not necessarily a usable URL. Keep only an
		// actual HTTP(S) link so a malformed field becomes empty during preview
		// instead of failing the database write later.
		row["deliverableLinks"] = ""
		if len(contentLinks) > 0 {
			row["deliverableLinks"] = contentLinks[0]
		}
		if excelCellString(row["influencer"]) == "" && excelCellString(row["deliverableLinks"]) == "" && excelCellString(row["platform"]) == "" && excelCellString(row["category"]) == "" {
			continue // 汇总行
		}
		inheritedInfluencer := excelCellString(row["influencer"]) == ""
		if inheritedInfluencer {
			row["influencer"] = previous["influencer"]
			if excelCellString(row["resourceName"]) == "" {
				row["resourceName"] = previous["resourceName"]
			}
			if excelCellString(row["resourceType"]) == "" {
				row["resourceType"] = previous["resourceType"]
			}
			if excelCellString(row["collaboratorTier"]) == "" {
				row["collaboratorTier"] = previous["collaboratorTier"]
			}
			if excelCellString(row["primaryContact"]) == "" {
				row["primaryContact"] = previous["primaryContact"]
			}
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
		errors := make([]string, 0)
		profileURL, profileErr := normalizeImportedProfileLink(excelCellString(row["influencer"]))
		if profileErr != nil {
			errors = append(errors, profileErr.Error())
		} else {
			row["influencer"] = profileURL
		}
		row["platform"] = normalizeImportedPlatform(excelCellString(row["platform"]), excelCellString(row["influencer"]))
		for _, field := range []string{"resourceType", "category", "platform", "cooperationType", "contentType"} {
			value := excelCellString(row[field])
			if value != "" && !standardImportOptionAllowed(options, field, value) {
				errors = append(errors, fmt.Sprintf("%s 必须使用平台预设选项", headerForImportField(field)))
			}
		}
		row["errors"] = errors
		row["country"] = normalizeImportedMarket(excelCellString(row["country"]))
		row["rowNo"] = rowIndex + 1
		row["sourceSheet"] = sheet
		resourceType := cleanImportString(excelCellString(row["resourceType"]))
		if resourceType == "" {
			resourceType = "KOL"
		}
		row["resourceType"] = resourceType
		row["mediaOutlet"] = ""
		if row["resourceType"] == "媒体" {
			row["mediaOutlet"] = firstNonEmpty(
				excelCellString(row["resourceName"]),
				importedProfilePlaceholderName(excelCellString(row["influencer"])),
			)
			if excelCellString(row["platform"]) == "" {
				row["platform"] = "Website"
			}
		}
		for field := range excelNumericFields {
			if _, exists := row[field]; !exists {
				row[field] = float64(0)
			}
		}
		// These values belong to the system side of the contract. Never trust
		// values typed into a locally-unprotected workbook.
		row["followerNumber"] = float64(0)
		row["collaboratorTier"] = ""
		row["views"] = float64(0)
		row["engagementCount"] = float64(0)
		row["cpm"] = float64(0)
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

func standardImportOptionAllowed(options map[string][]string, field, value string) bool {
	for _, option := range options[field] {
		if value == option {
			return true
		}
	}
	return false
}

func headerForImportField(field string) string {
	for _, definition := range excelContentAliases {
		if definition.field == field && len(definition.aliases) > 0 {
			return definition.aliases[0]
		}
	}
	return field
}

func standardImportRowMatches(values, expected []string) bool {
	for index, expectedValue := range expected {
		actual := ""
		if index < len(values) {
			actual = strings.TrimSpace(values[index])
		}
		if actual != expectedValue {
			return false
		}
	}
	for index := len(expected); index < len(values); index++ {
		if strings.TrimSpace(values[index]) != "" {
			return false
		}
	}
	return true
}

func standardImportInstructionRowMatches(values []string, label string) bool {
	return len(values) > 0 && strings.TrimSpace(values[0]) == label
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
	value := ""
	if column < len(row) {
		value = row[column]
	}
	if strings.TrimSpace(value) == "" {
		value = mergedValues[cell]
	}
	if visibleLinks := uniqueExcelLinks(httpExcelURLs(value)); len(visibleLinks) > 0 {
		// Excel may retain an old hyperlink relationship after the user edits
		// the displayed URL. In that case the visible value is the current
		// source of truth; combining both values would duplicate one sheet row.
		return visibleLinks
	}
	if ok, link, _ := book.GetCellHyperLink(sheet, cell); ok {
		return uniqueExcelLinks(httpExcelURLs(strings.ReplaceAll(link, "&amp;", "&")))
	}
	return nil
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
	return strings.TrimSpace(header) == alias
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
	value = strings.NewReplacer(
		",", "",
		"，", "",
		" ", "",
		"+", "",
		"$", "",
		"¥", "",
		"￥", "",
		"€", "",
		"£", "",
	).Replace(value)
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
	case strings.Contains(link, "facebook.com/") || strings.Contains(link, "fb.com/"):
		return "Facebook"
	case strings.Contains(link, "linkedin.com/"):
		return "LinkedIn"
	case strings.Contains(link, "reddit.com/"):
		return "Reddit"
	default:
		return "Website"
	}
}

func normalizeImportedPlatform(value, profileURL string) string {
	declared := cleanImportString(value)
	switch strings.ToLower(declared) {
	case "youtube":
		declared = "YouTube"
	case "tiktok":
		declared = "TikTok"
	case "instagram", "ins":
		declared = "Instagram"
	case "facebook", "fb":
		declared = "Facebook"
	case "twitter", "x":
		declared = "X"
	case "linkedin":
		declared = "LinkedIn"
	case "reddit":
		declared = "Reddit"
	case "website", "网站":
		declared = "Website"
	}
	inferred := platformFromLink(profileURL)
	if inferred != "Website" {
		return inferred
	}
	if declared != "" {
		return declared
	}
	return "Website"
}

func normalizeImportedProfileLink(value string) (string, error) {
	value = strings.TrimRight(cleanImportString(value), "，,。.;；、")
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("合作方必须填写完整有效的主页 URL")
	}
	query := parsed.Query()
	for key := range query {
		lowerKey := strings.ToLower(key)
		if strings.HasPrefix(lowerKey, "utm_") || lowerKey == "fbclid" || lowerKey == "gclid" || lowerKey == "igsh" {
			query.Del(key)
		}
	}
	parsed.RawQuery = query.Encode()
	parsed.Fragment = ""
	return parsed.String(), nil
}

func importedPlatformHandle(platform, profileURL string) string {
	switch platformDisplayName(platform) {
	case "X":
		return xHandleIdentifier(profileURL)
	default:
		return importedProfilePlaceholderName(profileURL)
	}
}

func importedProfilePlaceholderName(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Hostname() == "" {
		return strings.TrimSpace(value)
	}
	if handle := importedProfileAtHandle(parsed); handle != "" {
		return handle
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for index := len(segments) - 1; index >= 0; index-- {
		candidate := strings.TrimSpace(strings.TrimPrefix(segments[index], "@"))
		if candidate == "" || candidate == "channel" || candidate == "user" || candidate == "c" {
			continue
		}
		if host != "youtube.com" && host != "m.youtube.com" || !strings.EqualFold(candidate, "shorts") {
			return candidate
		}
	}
	return host
}

func importedProfileAtHandle(parsed *url.URL) string {
	if parsed == nil {
		return ""
	}
	for _, segment := range strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/") {
		decoded, err := url.PathUnescape(segment)
		if err != nil {
			decoded = segment
		}
		decoded = strings.TrimSpace(decoded)
		if !strings.HasPrefix(decoded, "@") {
			continue
		}
		if handle := strings.TrimSpace(strings.TrimPrefix(decoded, "@")); handle != "" {
			return handle
		}
	}
	return ""
}

func syncedResourceName(profileURL, fallback string) string {
	parsed, err := url.Parse(strings.TrimSpace(profileURL))
	if err == nil {
		if handle := importedProfileAtHandle(parsed); handle != "" {
			return handle
		}
	}
	return strings.TrimSpace(fallback)
}

func normalizeImportedMarket(value string) string {
	value = cleanImportString(value)
	key := strings.ToLower(strings.Join(strings.Fields(value), " "))
	aliases := map[string]string{
		"us": "美国", "u.s.": "美国", "usa": "美国", "united states": "美国", "united states of america": "美国", "美国": "美国",
		"uk": "英国", "u.k.": "英国", "united kingdom": "英国", "great britain": "英国", "英国": "英国",
		"ksa": "沙特阿拉伯", "saudi": "沙特阿拉伯", "saudi arabia": "沙特阿拉伯", "沙特": "沙特阿拉伯", "沙特阿拉伯": "沙特阿拉伯",
		"uae": "阿拉伯联合酋长国", "united arab emirates": "阿拉伯联合酋长国", "阿联酋": "阿拉伯联合酋长国", "阿拉伯联合酋长国": "阿拉伯联合酋长国",
		"de": "德国", "germany": "德国", "德国": "德国",
		"fr": "法国", "france": "法国", "法国": "法国",
		"jp": "日本", "japan": "日本", "日本": "日本",
		"kr": "韩国", "south korea": "韩国", "republic of korea": "韩国", "韩国": "韩国",
		"in": "印度", "india": "印度", "印度": "印度",
		"id": "印度尼西亚", "indonesia": "印度尼西亚", "印尼": "印度尼西亚", "印度尼西亚": "印度尼西亚",
		"sg": "新加坡", "singapore": "新加坡", "新加坡": "新加坡",
	}
	if canonical := aliases[key]; canonical != "" {
		return canonical
	}
	return value
}
