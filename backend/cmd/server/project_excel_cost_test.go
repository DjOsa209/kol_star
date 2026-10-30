package main

import (
	"math"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestParseExcelContentSheetPreservesPackageCost(t *testing.T) {
	for _, tt := range []struct {
		name  string
		links []string
		merge bool
		want  []float64
	}{
		{"merged package", []string{"https://youtube.com/watch?v=one", "https://youtube.com/watch?v=two", "https://youtube.com/watch?v=three"}, true, []float64{333.34, 333.33, 333.33}},
		{"merged duplicate content", []string{"https://youtube.com/watch?v=one", "https://youtube.com/watch?v=one", "https://youtube.com/watch?v=two"}, true, []float64{500, 500, 500}},
		{"multiple links in one cell", []string{"https://youtube.com/watch?v=one\nhttps://youtube.com/watch?v=two"}, false, []float64{500, 500}},
		{"independent equal amounts", []string{"https://youtube.com/watch?v=one", "https://youtube.com/watch?v=two"}, false, []float64{1000, 1000}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			book := excelize.NewFile()
			defer book.Close()
			sheet := book.GetSheetName(0)
			writeStandardImportHeaders(t, book, sheet)
			for i, link := range tt.links {
				values := []any{"", "Sample Creator", "https://youtube.com/@sample", "KOL", "科技", "美国", "", "", "YouTube", "付费合作", link, "兴趣圈层类", 1000}
				for column, value := range values {
					cell, _ := excelize.CoordinatesToCellName(column+1, i+5)
					if err := book.SetCellValue(sheet, cell, value); err != nil {
						t.Fatal(err)
					}
				}
			}
			if tt.merge {
				end, _ := excelize.CoordinatesToCellName(13, len(tt.links)+4)
				if err := book.MergeCell(sheet, "M5", end); err != nil {
					t.Fatal(err)
				}
			}
			rows, err := parseExcelContentSheet(book, sheet)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != len(tt.want) {
				t.Fatalf("rows=%d, want %d", len(rows), len(tt.want))
			}
			for i, row := range rows {
				if math.Abs(floatField(row, "quoteAmount")-tt.want[i]) > .001 {
					t.Errorf("row %d cost=%.2f, want %.2f", i, floatField(row, "quoteAmount"), tt.want[i])
				}
			}
		})
	}
}
