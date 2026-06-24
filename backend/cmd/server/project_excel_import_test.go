package main

import (
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestParseExcelContentSheetUsesCellHyperlink(t *testing.T) {
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	headers := []string{"姓名", "平台", "发布链接", "播放量"}
	for column, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		if err := book.SetCellValue(sheet, cell, header); err != nil {
			t.Fatal(err)
		}
	}
	_ = book.SetCellValue(sheet, "A2", "Creator One")
	_ = book.SetCellValue(sheet, "B2", "YouTube")
	_ = book.SetCellValue(sheet, "C2", "A visible video title")
	_ = book.SetCellValue(sheet, "D2", 1234)
	if err := book.SetCellHyperLink(sheet, "C2", "https://youtube.com/watch?v=abc", "External"); err != nil {
		t.Fatal(err)
	}

	rows, err := parseExcelContentSheet(book, sheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("parsed rows = %d, want 1", len(rows))
	}
	if got := rows[0]["deliverableLinks"]; got != "https://youtube.com/watch?v=abc" {
		t.Fatalf("link = %v", got)
	}
}

func TestParseExcelContentSheetMatchesMultilineRelatedHeaders(t *testing.T) {
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	headers := []string{
		"姓名\nInfluencer", "领域\nCategory", "平台\nPlatform",
		"粉丝数\nFollower Number", "发布日期\nRelease Date",
		"发布链接\nDeliverable Links", "播放量\nViews",
		"转赞藏数\nLikes+Fav+Share", "评论数\nComments",
	}
	for column, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		if err := book.SetCellValue(sheet, cell, header); err != nil {
			t.Fatal(err)
		}
	}
	values := []any{
		"Creator One", "Technology", "YouTube", "13,737", "2026-06-01",
		"https://youtube.com/watch?v=abc", "NaN", "1.2K", "42",
	}
	for column, value := range values {
		cell, _ := excelize.CoordinatesToCellName(column+1, 2)
		if err := book.SetCellValue(sheet, cell, value); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := parseExcelContentSheet(book, sheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("parsed rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row["influencer"] != "Creator One" || row["deliverableLinks"] != "https://youtube.com/watch?v=abc" {
		t.Fatalf("unexpected field mapping: %#v", row)
	}
	if row["followerNumber"] != float64(13737) || row["views"] != float64(0) || row["engagementCount"] != float64(1200) || row["commentsCount"] != float64(42) {
		t.Fatalf("unexpected numeric normalization: %#v", row)
	}
}

func TestExcelHeaderRowRejectsContentRowWithURL(t *testing.T) {
	values := []string{
		"News Channel Nebraska", "Media & Information",
		"https://example.com/published-link", "13,737",
	}
	if excelHeaderRow(values) {
		t.Fatal("content row was incorrectly recognized as a header")
	}
}

func TestParseExcelContentSheetCarriesForwardBlankCreatorFields(t *testing.T) {
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	for column, header := range []string{"KOL", "Platform", "Follower Number", "Link"} {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		if err := book.SetCellValue(sheet, cell, header); err != nil {
			t.Fatal(err)
		}
	}
	for column, value := range []any{"Creator One", "TikTok", "123,000", "https://www.tiktok.com/@creator/video/1"} {
		cell, _ := excelize.CoordinatesToCellName(column+1, 2)
		_ = book.SetCellValue(sheet, cell, value)
	}
	_ = book.SetCellValue(sheet, "D3", "https://www.tiktok.com/@creator/video/2")

	rows, err := parseExcelContentSheet(book, sheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1]["influencer"] != "Creator One" || rows[1]["platform"] != "TikTok" || rows[1]["followerNumber"] != float64(123000) {
		t.Fatalf("blank values were not carried forward: %#v", rows)
	}
}

func TestParseExcelContentSheetReadsMergedCreatorCells(t *testing.T) {
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	for column, header := range []string{"姓名", "平台", "发布链接"} {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		if err := book.SetCellValue(sheet, cell, header); err != nil {
			t.Fatal(err)
		}
	}
	_ = book.SetCellValue(sheet, "A2", "Creator One")
	_ = book.SetCellValue(sheet, "B2", "TikTok")
	_ = book.MergeCell(sheet, "A2", "A3")
	_ = book.MergeCell(sheet, "B2", "B3")
	_ = book.SetCellValue(sheet, "C2", "https://www.tiktok.com/@creator/video/1")
	_ = book.SetCellValue(sheet, "C3", "https://www.tiktok.com/@creator/video/2")

	rows, err := parseExcelContentSheet(book, sheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("parsed rows = %d, want 2", len(rows))
	}
	for _, row := range rows {
		if row["influencer"] != "Creator One" || row["platform"] != "TikTok" {
			t.Fatalf("merged values were not applied: %#v", row)
		}
	}
}

func TestParseExcelContentSheetKeepsEveryHyperlinkedRowUnderMergedCreator(t *testing.T) {
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	for column, header := range []string{"姓名", "平台", "粉丝数", "发布链接"} {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		if err := book.SetCellValue(sheet, cell, header); err != nil {
			t.Fatal(err)
		}
	}
	_ = book.SetCellValue(sheet, "A2", "Creator One")
	_ = book.SetCellValue(sheet, "B2", "YouTube")
	_ = book.SetCellValue(sheet, "B3", "YouTube")
	_ = book.SetCellValue(sheet, "B4", "Instagram")
	_ = book.SetCellValue(sheet, "C2", 569000)
	_ = book.MergeCell(sheet, "A2", "A4")
	_ = book.MergeCell(sheet, "C2", "C4")
	for row, link := range []string{
		"https://youtube.com/watch?v=first",
		"https://youtube.com/watch?v=second",
		"https://instagram.com/reel/third",
	} {
		cell := "D" + string(rune('2'+row))
		_ = book.SetCellValue(sheet, cell, "点击查看内容")
		if err := book.SetCellHyperLink(sheet, cell, link, "External"); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := parseExcelContentSheet(book, sheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("parsed rows = %d, want 3: %#v", len(rows), rows)
	}
	for index, row := range rows {
		if row["influencer"] != "Creator One" || row["followerNumber"] != float64(569000) {
			t.Fatalf("merged creator fields missing on row %d: %#v", index, row)
		}
	}
	if rows[0]["deliverableLinks"] != "https://youtube.com/watch?v=first" || rows[1]["deliverableLinks"] != "https://youtube.com/watch?v=second" || rows[2]["deliverableLinks"] != "https://instagram.com/reel/third" {
		t.Fatalf("hyperlink rows were not preserved: %#v", rows)
	}
}

func TestParseExcelContentSheetNormalizesInvalidFieldsBeforeImport(t *testing.T) {
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	for column, header := range []string{"姓名", "平台", "发布日期", "发布链接", "播放量"} {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		if err := book.SetCellValue(sheet, cell, header); err != nil {
			t.Fatal(err)
		}
	}
	_ = book.SetCellValue(sheet, "A2", "Creator One")
	_ = book.SetCellValue(sheet, "B2", "YouTube")
	_ = book.SetCellValue(sheet, "C2", "07/31/25")
	_ = book.SetCellValue(sheet, "D2", "not a valid URL")
	_ = book.SetCellValue(sheet, "E2", "not a number")

	rows, err := parseExcelContentSheet(book, sheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("parsed rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row["releaseDate"] != "2025-07-31" || row["deliverableLinks"] != "" || row["views"] != float64(0) {
		t.Fatalf("invalid fields were not normalized: %#v", row)
	}
}

func TestParseExcelContentSheetFindsContentURLInUnmappedColumn(t *testing.T) {
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	for column, header := range []string{"KOL", "Platform", "Relevant Coverage"} {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		if err := book.SetCellValue(sheet, cell, header); err != nil {
			t.Fatal(err)
		}
	}
	for column, value := range []any{
		"Creator One", "YouTube", "Coverage: https://youtube.com/watch?v=abc",
	} {
		cell, _ := excelize.CoordinatesToCellName(column+1, 2)
		if err := book.SetCellValue(sheet, cell, value); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := parseExcelContentSheet(book, sheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["deliverableLinks"] != "https://youtube.com/watch?v=abc" {
		t.Fatalf("unmapped content URL was not found: %#v", rows)
	}
}

func TestParseExcelContentSheetKeepsEveryContentURLInOneCell(t *testing.T) {
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	for column, header := range []string{"KOL", "Platform", "Relevant Coverage"} {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		if err := book.SetCellValue(sheet, cell, header); err != nil {
			t.Fatal(err)
		}
	}
	_ = book.SetCellValue(sheet, "A2", "Creator One")
	_ = book.SetCellValue(sheet, "B2", "YouTube")
	_ = book.SetCellValue(sheet, "C2", "https://youtube.com/watch?v=first\nhttps://youtube.com/watch?v=second")

	rows, err := parseExcelContentSheet(book, sheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("parsed rows = %d, want 2: %#v", len(rows), rows)
	}
	if rows[0]["deliverableLinks"] != "https://youtube.com/watch?v=first" || rows[1]["deliverableLinks"] != "https://youtube.com/watch?v=second" {
		t.Fatalf("content links were not split: %#v", rows)
	}
}

func TestParseExcelContentSheetSavesHyperlinkTargetFromDisplayText(t *testing.T) {
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	for column, header := range []string{"KOL", "Platform", "Material"} {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		if err := book.SetCellValue(sheet, cell, header); err != nil {
			t.Fatal(err)
		}
	}
	_ = book.SetCellValue(sheet, "A2", "Creator One")
	_ = book.SetCellValue(sheet, "B2", "YouTube")
	_ = book.SetCellValue(sheet, "C2", "点击查看作品")
	if err := book.SetCellHyperLink(sheet, "C2", "https://youtube.com/watch?v=abc&feature=share", "External"); err != nil {
		t.Fatal(err)
	}

	rows, err := parseExcelContentSheet(book, sheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["deliverableLinks"] != "https://youtube.com/watch?v=abc&feature=share" {
		t.Fatalf("hyperlink target was not saved: %#v", rows)
	}
}

func TestParseExcelContentSheetRecognizesPublicationAndURLColumns(t *testing.T) {
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	headers := []string{"Campaign", "Publication", "Date", "Headline", "URL"}
	for column, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		if err := book.SetCellValue(sheet, cell, header); err != nil {
			t.Fatal(err)
		}
	}
	values := []any{
		"CES", "Android Headlines", "2026/01/06",
		"Infinix at CES", "https://www.androidheadlines.com/infinix-ces",
	}
	for column, value := range values {
		cell, _ := excelize.CoordinatesToCellName(column+1, 2)
		if err := book.SetCellValue(sheet, cell, value); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := parseExcelContentSheet(book, sheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("parsed rows = %d, want 1", len(rows))
	}
	if rows[0]["influencer"] != "Android Headlines" || rows[0]["deliverableLinks"] != "https://www.androidheadlines.com/infinix-ces" || rows[0]["platform"] != "Website" {
		t.Fatalf("unexpected publication row: %#v", rows[0])
	}
}

func TestParseExcelContentSheetKeepsProfilesWithoutPublishedLinks(t *testing.T) {
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	for column, header := range []string{"Tier", "Category", "Media Outlet", "Country"} {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		if err := book.SetCellValue(sheet, cell, header); err != nil {
			t.Fatal(err)
		}
	}
	for column, value := range []any{"1", "Tech Media", "Example Media", "US"} {
		cell, _ := excelize.CoordinatesToCellName(column+1, 2)
		if err := book.SetCellValue(sheet, cell, value); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := parseExcelContentSheet(book, sheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("parsed rows = %d, want 1", len(rows))
	}
	if rows[0]["influencer"] != "Example Media" || rows[0]["resourceType"] != "媒体" || excelCellString(rows[0]["deliverableLinks"]) != "" {
		t.Fatalf("unexpected profile row: %#v", rows[0])
	}
}
