package main

import (
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func writeStandardImportHeaders(t *testing.T, book *excelize.File, sheet string) {
	t.Helper()
	for rowIndex, values := range [][]string{
		standardProjectImportHeaders, standardProjectImportLabels, standardProjectImportScopes, standardProjectImportRules,
	} {
		for column, value := range values {
			cell, _ := excelize.CoordinatesToCellName(column+1, rowIndex+1)
			if err := book.SetCellValue(sheet, cell, value); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestParseExcelContentSheetAcceptsLockedTwoRowStandardHeader(t *testing.T) {
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	writeStandardImportHeaders(t, book, sheet)
	values := []any{
		"", "https://youtube.com/@creatorone", "KOL", "科技", "USA", "13.7K", "头部", "YouTube",
		"付费合作", "https://youtube.com/watch?v=abc", "2000", "100000", "1200",
		"creator@example.com", "Mia", "Vendor A", "标准模板导入", "999",
	}
	for column, value := range values {
		cell, _ := excelize.CoordinatesToCellName(column+1, 5)
		if err := book.SetCellValue(sheet, cell, value); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := parseExcelContentSheet(book, sheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("parsed rows = %d, want 1: %#v", len(rows), rows)
	}
	row := rows[0]
	if row["rowNo"] != 5 || row["influencer"] != "https://youtube.com/@creatorone" || row["resourceType"] != "KOL" || row["country"] != "美国" {
		t.Fatalf("unexpected resource mapping: %#v", row)
	}
	if row["cooperationType"] != "付费合作" || row["primaryContact"] != "creator@example.com" || row["owner"] != "Mia" || row["vendor"] != "Vendor A" {
		t.Fatalf("unexpected cooperation mapping: %#v", row)
	}
	if row["followerNumber"] != float64(0) || row["views"] != float64(0) || row["engagementCount"] != float64(0) || row["cpm"] != float64(0) {
		t.Fatalf("system-owned metrics must ignore workbook values: %#v", row)
	}
}

func TestParseExcelContentSheetRejectsOneRowLegacyTemplate(t *testing.T) {
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	for column, header := range standardProjectImportHeaders[1:] {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		_ = book.SetCellValue(sheet, cell, header)
	}
	_ = book.SetCellValue(sheet, "A2", "Legacy Creator")

	if _, err := parseExcelContentSheet(book, sheet); err == nil {
		t.Fatal("one-row legacy template must be rejected")
	}
}

func TestParseExcelContentSheetRejectsChangedChineseHeader(t *testing.T) {
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	writeStandardImportHeaders(t, book, sheet)
	_ = book.SetCellValue(sheet, "B2", "达人名称")

	if _, err := parseExcelContentSheet(book, sheet); err == nil {
		t.Fatal("changed second header row must be rejected")
	}
}

func TestParseExcelContentSheetAcceptsChangedInstructionCopy(t *testing.T) {
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	writeStandardImportHeaders(t, book, sheet)
	for cell, value := range map[string]string{
		"F3": "KOL：粉丝数、订阅数；\n\n媒体：月独立访客（UMV,unique visitors per month ）、月访问量；",
		"G3": "头部\n腰部\n尾部",
		"L3": "播放量\n曝光量\n阅读量",
		"M3": "总互动量（点赞、评论、分享、收藏）",
		"F4": "填写纯数字，不带K/M。根据资源类型自动识别对应指标（Followers、UMV、Members等）。",
		"G4": "/",
		"L4": "优先填写实际播放量（Views）",
		"M4": "填写总互动数（Likes + Comments + Shares + Saves）",
		"R4": "",
	} {
		_ = book.SetCellValue(sheet, cell, value)
	}

	if _, err := parseExcelContentSheet(book, sheet); err != nil {
		t.Fatalf("instruction copy must not invalidate standard headers: %v", err)
	}
}

func TestParseExcelContentSheetRejectsMissingInstructionStructure(t *testing.T) {
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	writeStandardImportHeaders(t, book, sheet)
	_ = book.SetCellValue(sheet, "A3", "其他说明")

	if _, err := parseExcelContentSheet(book, sheet); err == nil {
		t.Fatal("instruction row labels must remain in the standard positions")
	}
}

func TestParseExcelContentSheetKeepsEveryStandardContentURL(t *testing.T) {
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	writeStandardImportHeaders(t, book, sheet)
	_ = book.SetCellValue(sheet, "B5", "https://youtube.com/@creatorone")
	_ = book.SetCellValue(sheet, "H5", "YouTube")
	_ = book.SetCellValue(sheet, "J5", "https://youtube.com/watch?v=one https://youtube.com/watch?v=two")

	rows, err := parseExcelContentSheet(book, sheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("parsed rows = %d, want 2: %#v", len(rows), rows)
	}
}

func TestBuildStandardProjectImportTemplateHasProtectedTwoRowHeader(t *testing.T) {
	book, err := buildStandardProjectImportTemplate()
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	sheet := "标准模板"
	for column, expected := range standardProjectImportHeaders {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		if actual, _ := book.GetCellValue(sheet, cell); actual != expected {
			t.Fatalf("%s = %q, want %q", cell, actual, expected)
		}
	}
	for column, expected := range standardProjectImportLabels {
		if column == 0 || column == len(standardProjectImportLabels)-1 {
			continue // merged into A1 and R1
		}
		cell, _ := excelize.CoordinatesToCellName(column+1, 2)
		if actual, _ := book.GetCellValue(sheet, cell); actual != expected {
			t.Fatalf("%s = %q, want %q", cell, actual, expected)
		}
	}
	for row, expectedValues := range map[int][]string{3: standardProjectImportScopes, 4: standardProjectImportRules} {
		for column, expected := range expectedValues {
			cell, _ := excelize.CoordinatesToCellName(column+1, row)
			if actual, _ := book.GetCellValue(sheet, cell); actual != expected {
				t.Fatalf("%s = %q, want %q", cell, actual, expected)
			}
		}
	}
	headerStyleID, _ := book.GetCellStyle(sheet, "B1")
	headerStyle, _ := book.GetStyle(headerStyleID)
	instructionStyleID, _ := book.GetCellStyle(sheet, "B3")
	instructionStyle, _ := book.GetStyle(instructionStyleID)
	dataStyleID, _ := book.GetCellStyle(sheet, "B5")
	dataStyle, _ := book.GetStyle(dataStyleID)
	calculatedStyleID, _ := book.GetCellStyle(sheet, "F5")
	calculatedStyle, _ := book.GetStyle(calculatedStyleID)
	costStyleID, _ := book.GetCellStyle(sheet, "K5")
	costStyle, _ := book.GetStyle(costStyleID)
	if headerStyle.Protection == nil || !headerStyle.Protection.Locked {
		t.Fatal("header must be locked")
	}
	if dataStyle.Protection == nil || dataStyle.Protection.Locked {
		t.Fatal("data cells must remain editable")
	}
	if calculatedStyle.Protection == nil || !calculatedStyle.Protection.Locked {
		t.Fatal("system-calculated data cells must be locked")
	}
	if costStyle.Protection == nil || costStyle.Protection.Locked {
		t.Fatal("cost cells must remain editable")
	}
	if costStyle.CustomNumFmt == nil || *costStyle.CustomNumFmt != standardProjectCostNumberFormat {
		t.Fatalf("cost number format = %#v, want %q", costStyle.CustomNumFmt, standardProjectCostNumberFormat)
	}
	if instructionStyle.Protection == nil || !instructionStyle.Protection.Locked {
		t.Fatal("instruction rows must be locked")
	}
	panes, err := book.GetPanes(sheet)
	if err != nil || !panes.Freeze || panes.YSplit != 4 {
		t.Fatalf("expected first four rows frozen: %#v, %v", panes, err)
	}
	merges, err := book.GetMergeCells(sheet)
	if err != nil {
		t.Fatal(err)
	}
	merged := map[string]bool{}
	for _, merge := range merges {
		merged[merge.GetStartAxis()+":"+merge.GetEndAxis()] = true
	}
	if !merged["A1:A2"] || !merged["R1:R2"] {
		t.Fatalf("required header merges missing: %#v", merged)
	}
}

func TestDownloadedStandardTemplateCanBeUploadedWithoutHeaderMismatch(t *testing.T) {
	book, err := buildStandardProjectImportTemplate()
	if err != nil {
		t.Fatal(err)
	}
	buffer, err := book.WriteToBuffer()
	book.Close()
	if err != nil {
		t.Fatal(err)
	}

	uploaded, err := excelize.OpenReader(buffer)
	if err != nil {
		t.Fatal(err)
	}
	defer uploaded.Close()
	if _, err := parseExcelContentSheet(uploaded, "标准模板"); err != nil {
		t.Fatalf("downloaded standard template must be accepted unchanged: %v", err)
	}
}

func TestBuildStandardProjectExportWorkbookMatchesImportContract(t *testing.T) {
	book, err := buildStandardProjectExportWorkbook(defaultStandardProjectImportOptions, []map[string]any{
		{
			"resourceName": "Creator One", "profileUrl": "https://www.instagram.com/creatorone/",
			"resourceType": "KOL", "category": "科技", "market": "美国", "audienceSize": float64(1250000),
			"collaboratorTier": "头部", "platform": "Instagram", "cooperationType": "付费合作",
			"contentUrl": "https://www.instagram.com/p/example/", "cost": float64(2500),
			"views": float64(500000), "engagement": float64(23000), "primaryContact": "creator@example.com",
			"owner": "Mia", "vendor": "Vendor A", "notes": "项目导出", "cpm": float64(5),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()

	if actual, _ := book.GetCellValue("标准模板", "B5"); actual != "Creator One" {
		t.Fatalf("B5 = %q, want creator display name", actual)
	}
	if ok, target, _ := book.GetCellHyperLink("标准模板", "B5"); !ok || target != "https://www.instagram.com/creatorone/" {
		t.Fatalf("profile hyperlink = %q, ok=%v", target, ok)
	}
	if ok, target, _ := book.GetCellHyperLink("标准模板", "J5"); !ok || target != "https://www.instagram.com/p/example/" {
		t.Fatalf("content hyperlink = %q, ok=%v", target, ok)
	}
	for cell, expected := range map[string]string{"F5": "1250000", "K5": "2500", "L5": "500000", "M5": "23000", "R5": "5"} {
		if actual, _ := book.GetCellValue("标准模板", cell, excelize.Options{RawCellValue: true}); actual != expected {
			t.Fatalf("%s = %q, want %q", cell, actual, expected)
		}
	}
	if actual, _ := book.GetCellValue("标准模板", "K5"); actual != "$2,500.00" {
		t.Fatalf("formatted K5 = %q, want $2,500.00", actual)
	}
	if err := book.SetCellValue("标准模板", "K6", -1234.1); err != nil {
		t.Fatal(err)
	}
	if actual, _ := book.GetCellValue("标准模板", "K6"); actual != "-$1,234.10" {
		t.Fatalf("formatted negative K6 = %q, want -$1,234.10", actual)
	}

	parsed, err := parseExcelContentSheet(book, "标准模板")
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 1 {
		t.Fatalf("parsed rows = %d, want 1: %#v", len(parsed), parsed)
	}
	if parsed[0]["influencer"] != "https://www.instagram.com/creatorone/" || parsed[0]["deliverableLinks"] != "https://www.instagram.com/p/example/" {
		t.Fatalf("export must be re-importable: %#v", parsed[0])
	}
	if got := floatField(parsed[0], "quoteAmount"); got != 2500 {
		t.Fatalf("parsed cooperation cost = %v, want 2500: %#v", got, parsed[0])
	}
}

func TestBuildStandardProjectExportRowsIncludesAllProjectRelationships(t *testing.T) {
	project := map[string]any{"targetMarket": "美国", "owner": "Project Owner"}
	resources := []map[string]any{
		{"resourceId": 1, "resourceName": "Creator", "resourceType": "KOL", "platform": "TikTok", "platformHandle": "creator", "followers": 1500000},
		{"resourceId": 2, "resourceName": "Media", "resourceType": "媒体", "platform": "Website", "platformUrl": "https://example.com", "audienceSize": 220000},
	}
	cooperations := []map[string]any{
		{"resourceId": 1, "resourceName": "Creator", "resourceType": "KOL", "platform": "TikTok", "platformHandle": "creator", "followers": 1500000, "finalLink": "https://www.tiktok.com/@creator/video/1", "quoteAmount": 1000, "views": 500000},
		{"resourceId": 1, "resourceName": "Creator", "resourceType": "KOL", "platform": "TikTok", "platformHandle": "creator", "followers": 1500000, "finalLink": "https://www.tiktok.com/@creator/video/2", "quoteAmount": 500, "views": 250000},
	}

	rows := buildStandardProjectExportRows(project, resources, cooperations)
	if len(rows) != 3 {
		t.Fatalf("export rows = %d, want two cooperations plus one unlinked resource: %#v", len(rows), rows)
	}
	if rows[0]["profileUrl"] != "https://www.tiktok.com/@creator" || rows[0]["collaboratorTier"] != "头部" || rows[0]["cpm"] != float64(2) {
		t.Fatalf("unexpected creator export row: %#v", rows[0])
	}
	if rows[2]["resourceName"] != "Media" || rows[2]["market"] != "美国" || rows[2]["collaboratorTier"] != "腰部" {
		t.Fatalf("unlinked resource missing or incorrect: %#v", rows[2])
	}
}

func TestNormalizeImportedProfileLogic(t *testing.T) {
	profile, err := normalizeImportedProfileLink("https://www.instagram.com/example/?utm_source=test#bio")
	if err != nil {
		t.Fatal(err)
	}
	if profile != "https://www.instagram.com/example/" {
		t.Fatalf("profile = %q", profile)
	}
	if got := normalizeImportedPlatform("Website", profile); got != "Instagram" {
		t.Fatalf("platform = %q, want Instagram", got)
	}
	if got := importedProfilePlaceholderName(profile); got != "example" {
		t.Fatalf("placeholder = %q, want example", got)
	}
	if got := importedProfilePlaceholderName("https://youtube.com/@creator-name/videos"); got != "creator-name" {
		t.Fatalf("YouTube placeholder = %q, want creator-name", got)
	}
	if got := syncedResourceName("https://www.tiktok.com/@creator/video/123", "Creator Display Name"); got != "creator" {
		t.Fatalf("synced resource name = %q, want creator", got)
	}
	if got := syncedResourceName("https://example.com/profile", "Creator Display Name"); got != "Creator Display Name" {
		t.Fatalf("fallback resource name = %q, want Creator Display Name", got)
	}
	if got := normalizeImportedMarket("Saudi"); got != "沙特阿拉伯" {
		t.Fatalf("market = %q, want 沙特阿拉伯", got)
	}
}

func TestIncrementalImportQueuesMatchedResourceForPlatformSync(t *testing.T) {
	resourceIDs := map[int64]bool{}
	queueImportedResourceForSync(resourceIDs, 42)
	if !resourceIDs[42] {
		t.Fatal("matched resource from incremental import was not queued for platform sync")
	}
}

func TestDuplicateImportStillStartsMatchedResourceSync(t *testing.T) {
	if !shouldStartImportedProjectSync(1) {
		t.Fatal("duplicate import with a matched resource did not start platform sync")
	}
}

func TestProjectNameFromUploadFileName(t *testing.T) {
	for _, test := range []struct {
		fileName string
		want     string
	}{
		{fileName: "Infinix NOTE 60 Series Master Media List.xlsx", want: "Infinix NOTE 60 Series Master Media List"},
		{fileName: "campaign.v2.csv", want: "campaign.v2"},
		{fileName: "/tmp/project.xls", want: "project"},
	} {
		if got := projectNameFromUploadFileName(test.fileName); got != test.want {
			t.Fatalf("projectNameFromUploadFileName(%q) = %q, want %q", test.fileName, got, test.want)
		}
	}
}

func TestExcelNumberValueAcceptsFormattedCurrency(t *testing.T) {
	for input, want := range map[string]float64{
		"$2,500.00": 2500,
		"￥1,234.50": 1234.5,
		"€99":       99,
	} {
		if got := excelNumberValue(input); got != want {
			t.Fatalf("excelNumberValue(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestDynamicStandardOptionsDriveTemplateAndParser(t *testing.T) {
	options := cloneStandardImportOptions(defaultStandardProjectImportOptions)
	options["category"] = append(options["category"], "新能源")
	book, err := buildStandardProjectImportTemplateWithOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	validations, err := book.GetDataValidations("标准模板")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, validation := range validations {
		if validation.Sqref == "D5:D2000" && strings.Contains(validation.Formula1, "新能源") {
			found = true
		}
	}
	if !found {
		t.Fatal("dynamic category must be included in the template drop-down")
	}
	_ = book.SetCellValue("标准模板", "B5", "https://youtube.com/@creator")
	_ = book.SetCellValue("标准模板", "C5", "KOL")
	_ = book.SetCellValue("标准模板", "D5", "新能源")
	_ = book.SetCellValue("标准模板", "H5", "YouTube")
	rows, err := parseExcelContentSheetWithOptions(book, "标准模板", options)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || len(rows[0]["errors"].([]string)) != 0 {
		t.Fatalf("dynamic option should parse without errors: %#v", rows)
	}
}
