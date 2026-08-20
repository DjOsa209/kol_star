package main

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNormalizeImportedCooperationLink(t *testing.T) {
	got, err := normalizeImportedCooperationLink("https://example.com/review?utm_source=x&keep=1#section")
	if err != nil {
		t.Fatalf("normalize link: %v", err)
	}
	if want := "https://example.com/review?keep=1"; got != want {
		t.Fatalf("normalized link = %q, want %q", got, want)
	}

	if _, err := normalizeImportedCooperationLink("not-a-link"); err == nil {
		t.Fatal("expected invalid link to be rejected")
	}
}

func TestImportedLinkWebsite(t *testing.T) {
	got := importedLinkWebsite("https://www.youtube.com/watch?v=abc&utm_source=sheet")
	if want := "https://www.youtube.com"; got != want {
		t.Fatalf("website = %q, want %q", got, want)
	}
}

func TestImportedReleaseDate(t *testing.T) {
	cases := map[string]any{
		"2025-08-13": "2025-08-13",
		"07/31/25":   "2025-07-31",
		"08/13/25":   "2025-08-13",
		"10-25-09":   "2009-10-25",
		"not a date": nil,
	}
	for input, want := range cases {
		if got := importedReleaseDate(input); got != want {
			t.Fatalf("importedReleaseDate(%q) = %#v, want %#v", input, got, want)
		}
	}
}

func TestNormalizePlatformResourceName(t *testing.T) {
	if got := normalizePlatformResourceName("Chigz Tech · Official"); got != "chigztechofficial" {
		t.Fatalf("normalized name = %q", got)
	}
}

func TestImportCooperationRowKeyNormalizesTrackingParameters(t *testing.T) {
	base := map[string]any{
		"influencer":       "https://www.instagram.com/creator/?utm_source=sheet",
		"platform":         "Instagram",
		"deliverableLinks": "https://www.instagram.com/p/POST/?utm_campaign=launch",
		"cooperationType":  "付费合作",
		"quoteAmount":      float64(1200),
		"owner":            "Mia",
		"vendor":           "Vendor A",
	}
	same := map[string]any{}
	for key, value := range base {
		same[key] = value
	}
	same["influencer"] = "https://www.instagram.com/creator/"
	same["deliverableLinks"] = "https://www.instagram.com/p/POST/"

	first, err := importCooperationRowKey(base)
	if err != nil {
		t.Fatal(err)
	}
	second, err := importCooperationRowKey(same)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("equivalent rows must share a dedupe key:\n%s\n%s", first, second)
	}

	same["quoteAmount"] = float64(1500)
	changed, err := importCooperationRowKey(same)
	if err != nil {
		t.Fatal(err)
	}
	if changed == first {
		t.Fatal("different cooperation data must not be treated as the same row")
	}
}

func TestInsertImportCooperationStoresContentType(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(`insert into biz_cooperations\s+\(project_id, resource_id, cooperation_type, content_type`).
		WithArgs(11, int64(22), "付费合作", "消费种草类", "Mia", "Vendor A", float64(1200), "IMP-1", "新品合作").
		WillReturnResult(sqlmock.NewResult(33, 1))
	mock.ExpectCommit()

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	action, err := insertImportCooperation(context.Background(), tx, 11, 22, "IMP-1", map[string]any{
		"cooperationType": "付费合作",
		"contentType":     "消费种草类",
		"owner":           "Mia",
		"vendor":          "Vendor A",
		"quoteAmount":     float64(1200),
		"notes":           "新品合作",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if action != importCooperationCreated {
		t.Fatalf("action = %q, want %q", action, importCooperationCreated)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
