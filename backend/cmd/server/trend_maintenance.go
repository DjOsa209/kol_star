package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Deployment maintenance uses the normal runtime configuration. Credentials are
// consumed by the database/cache clients and never included in its output.
func runTrendMaintenance(command string) error {
	cfg, _, err := loadConfig()
	if err != nil {
		return errors.New("runtime configuration unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	report := map[string]any{"backendAddress": cfg.Server.Addr, "database": cfg.MySQL.Database, "redisAddress": cfg.Redis.Addr, "redisDatabase": cfg.Redis.Database, "redisCredentialsConfigured": cfg.Redis.Password != "", "redisTLS": cfg.Redis.TLS}
	_, passwordFromEnvironment := os.LookupEnv("REDIS_PASSWORD")
	report["redisPasswordFromEnvironment"] = passwordFromEnvironment
	report["redisUsernameConfigured"] = cfg.Redis.Username != ""
	_, redisErr := (redisTrendCache{config: cfg.Redis}).Get(ctx, cfg.Redis.KeyPrefix+"health-probe")
	report["redisConnected"] = redisErr == nil
	if redisErr != nil {
		category := "unavailable"
		switch redisErr.Error() {
		case "redis authentication failed":
			category = "authentication_failed"
		case "redis database unavailable":
			category = "database_unavailable"
		case "redis rejected cache command":
			category = "command_rejected"
		case "remote redis requires TLS":
			category = "tls_required"
		}
		report["redisErrorCategory"] = category
	}
	db, err := openDB(cfg.MySQL)
	if err != nil {
		report["databaseConnected"] = false
		_ = json.NewEncoder(os.Stdout).Encode(report)
		return errors.New("configured database unavailable")
	}
	defer db.Close()
	report["databaseConnected"] = true
	a := newApp(db, cfg)
	before, err := a.queryMaps(ctx, `select id,parent_id,title,path,name,component,`+"`rank`"+`,icon,auths,show_link,menu_type from sys_menus where id=910 or path='/business/trends'`)
	if err != nil {
		return errors.New("menu preflight unavailable")
	}
	conflict := false
	for _, m := range before {
		if intField(m, "id") != 910 || stringField(m, "path") != trendMenu {
			conflict = true
		}
	}
	report["menuConflict"] = conflict
	report["menuPresent"] = len(before) == 1 && !conflict
	if command == "trends-migrate" {
		if conflict || cfg.MySQL.Database != "kol_admin" {
			_ = json.NewEncoder(os.Stdout).Encode(report)
			return errors.New("migration target conflict")
		}
		grants, err := a.queryMaps(ctx, `select role_id,menu_id from sys_role_menus where menu_id=910`)
		if err != nil {
			return errors.New("grant preflight unavailable")
		}
		dir := filepath.Join("..", "outputs", "operations")
		if err := os.MkdirAll(dir, 0700); err != nil {
			return errors.New("menu backup unavailable")
		}
		backup := filepath.Join(dir, "trends-menu-before-"+time.Now().UTC().Format("20060102T150405Z")+".json")
		data, err := json.MarshalIndent(map[string]any{"menus": before, "grants": grants}, "", "  ")
		if err != nil {
			return errors.New("menu backup unavailable")
		}
		if err := os.WriteFile(backup, data, 0600); err != nil {
			return errors.New("menu backup unavailable")
		}
		migration, err := os.ReadFile(filepath.Join("migrations", "058_trend_insights.sql"))
		if err != nil {
			return errors.New("migration file unavailable")
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return errors.New("migration transaction unavailable")
		}
		defer tx.Rollback()
		for _, statement := range strings.Split(string(migration), ";") {
			statement = strings.TrimSpace(statement)
			if statement == "" || strings.EqualFold(statement, "use kol_admin") {
				continue
			}
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return errors.New("menu migration failed")
			}
		}
		if err := tx.Commit(); err != nil {
			return errors.New("menu migration commit failed")
		}
		report["migrationApplied"] = true
		report["backupFile"] = filepath.Base(backup)
	}
	var menuCount, grantCount int
	if err := db.QueryRowContext(ctx, `select count(*) from sys_menus where id=910 and parent_id=900 and path='/business/trends' and component='business/trends/index'`).Scan(&menuCount); err != nil {
		return errors.New("menu verification failed")
	}
	if err := db.QueryRowContext(ctx, `select count(*) from sys_role_menus where menu_id=910`).Scan(&grantCount); err != nil {
		return errors.New("menu verification failed")
	}
	report["menuPresent"] = menuCount == 1
	report["roleGrantCount"] = grantCount
	return json.NewEncoder(os.Stdout).Encode(report)
}
