package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"
)

// Explicit opt-in: real configured model, isolated temporary database, no tenant messages.
func TestIPContentTranslationLiveDB(t *testing.T) {
	if os.Getenv("IP_TRANSLATION_LIVE_DB") != "1" {
		t.Skip("live translation database test is opt-in")
	}
	cfg, _, err := loadConfig()
	if err != nil {
		t.Fatal("configuration unavailable")
	}
	base, err := openDB(cfg.MySQL)
	if err != nil {
		t.Fatal("database unavailable")
	}
	t.Cleanup(func() { _ = base.Close() })
	if !regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString(cfg.MySQL.Database) {
		t.Fatal("unsupported schema identifier")
	}
	temp := fmt.Sprintf("ip_translation_test_%d", time.Now().UnixNano())
	if _, err := base.Exec("create database `" + temp + "` character set utf8mb4"); err != nil {
		t.Fatal("cannot create isolated translation database")
	}
	t.Cleanup(func() {
		if _, err := base.Exec("drop database `" + temp + "`"); err != nil {
			t.Errorf("clean isolated database: %v", err)
		}
	})
	for _, table := range []string{"biz_ip_resources", "biz_ip_requests", "biz_ip_request_candidates", "biz_localized_texts", "biz_governance_rules"} {
		if _, err := base.Exec("create table `" + temp + "`.`" + table + "` like `" + cfg.MySQL.Database + "`.`" + table + "`"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := base.Exec("insert into `" + temp + "`.biz_governance_rules select * from `" + cfg.MySQL.Database + "`.biz_governance_rules where rule_type='ai_model' limit 1"); err != nil {
		t.Fatal(err)
	}
	cfg.MySQL.Database = temp
	db, err := openDB(cfg.MySQL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newApp(db, cfg)
	ctx := context.Background()
	const chinese = "面向年轻用户的手机新品联名，授权范围与档期需要进一步确认。"
	const english = "Young adults who enjoy creative design and immersive experiences."
	result, err := db.Exec("insert into biz_ip_resources(name,ip_type,summary,audience) values ('Translation QA','文化艺术',?,?)", chinese, english)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()
	a.queueIPContent(ctx, "ip_resource", id)
	if err := a.processIPContentTranslations(ctx); err != nil {
		t.Fatal(err)
	}
	rows, err := a.ipContentRows(ctx, "ip_resource", id)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.attachIPContentTranslations(ctx, "ip_resource", rows); err != nil {
		t.Fatal(err)
	}
	meta := rows[0]["translation"].(*ipContentTranslation)
	if meta.State != "completed" || containsHan(meta.Values["en"]["summary"]) || !containsHan(meta.Values["zh"]["audience"]) || meta.Values["en"]["summary"] == "" {
		t.Fatalf("real bilingual generation incomplete: state=%s", meta.State)
	}
	if rows[0]["summary"] != chinese || rows[0]["audience"] != english {
		t.Fatal("original content overwritten")
	}
	t.Logf("Chinese → English: %s", meta.Values["en"]["summary"])
	t.Logf("English → Chinese: %s", meta.Values["zh"]["audience"])
	a.queueIPContent(ctx, "ip_resource", id)
	var pending int
	if err := db.QueryRow("select count(*) from biz_localized_texts where translation_status<>'completed'").Scan(&pending); err != nil || pending != 0 {
		t.Fatal("unchanged sources were unnecessarily queued")
	}
	const revised = "An international product collaboration; licensing terms remain subject to confirmation."
	if _, err := db.Exec("update biz_ip_resources set summary=? where id=?", revised, id); err != nil {
		t.Fatal(err)
	}
	a.queueIPContent(ctx, "ip_resource", id)
	if err := a.processIPContentTranslations(ctx); err != nil {
		t.Fatal(err)
	}
	rows, err = a.ipContentRows(ctx, "ip_resource", id)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.attachIPContentTranslations(ctx, "ip_resource", rows); err != nil {
		t.Fatal(err)
	}
	meta = rows[0]["translation"].(*ipContentTranslation)
	if rows[0]["summary"] != revised || meta.Values["en"]["summary"] != "" || !containsHan(meta.Values["zh"]["summary"]) || meta.State != "completed" {
		t.Fatal("source language change or stale-cache guard failed")
	}
	t.Log("PASS: real Chinese/English generation, source preservation, unchanged-source cache and source-language change")
	result, err = db.Exec(`insert into biz_ip_requests(project_name,department,goal,description,brief,marketing_profile) values ('Translation QA','测试','新品发布',?, ?, ?)`, chinese, `{"projectNotes":"需确认授权渠道与区域"}`, `{"functions":{"brand":{"comments":"The visual direction fits the brand; licensing terms still need review."},"imc":{"comments":"建议优先规划新品发布传播"}},"candidates":{}}`)
	if err != nil {
		t.Fatal(err)
	}
	requestID, _ := result.LastInsertId()
	result, err = db.Exec(`insert into biz_ip_request_candidates(request_id,ip_id,reason,assessment) values (?,?,?,?)`, requestID, id, "适合产品设计与礼盒联名", `{"notes":"Scope and timing need confirmation."}`)
	if err != nil {
		t.Fatal(err)
	}
	candidateID, _ := result.LastInsertId()
	a.queueIPContent(ctx, "ip_request", requestID)
	if err := a.processIPContentTranslations(ctx); err != nil {
		t.Fatal(err)
	}
	for _, record := range []struct {
		kind   string
		id     int64
		enPath string
		zhPath string
	}{
		{"ip_request", requestID, "description", "marketingProfile.functions.brand.comments"},
		{"ip_candidate", candidateID, "reason", "assessment.notes"},
	} {
		items, err := a.ipContentRows(ctx, record.kind, record.id)
		if err != nil {
			t.Fatal(err)
		}
		if err := a.attachIPContentTranslations(ctx, record.kind, items); err != nil {
			t.Fatal(err)
		}
		meta := items[0]["translation"].(*ipContentTranslation)
		if meta.State != "completed" || meta.Values["en"][record.enPath] == "" || containsHan(meta.Values["en"][record.enPath]) || !containsHan(meta.Values["zh"][record.zhPath]) {
			t.Fatalf("real request/opinion translation incomplete: %s %s", record.kind, meta.State)
		}
		if !containsHan(ipTranslationValue(items[0], record.enPath)) || containsHan(ipTranslationValue(items[0], record.zhPath)) {
			t.Fatal("request/opinion original content overwritten")
		}
	}
	t.Log("PASS: request description, IP opinion and independent marketing-function comments translated in both directions")
}
