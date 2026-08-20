package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	Server       ServerConfig      `mapstructure:"server"`
	MySQL        MySQLConfig       `mapstructure:"mysql"`
	CORS         CORSConfig        `mapstructure:"cors"`
	PlatformAPIs PlatformAPIConfig `mapstructure:"platform_apis"`
	Collector    CollectorConfig   `mapstructure:"collector"`
	AIModel      AIModelConfig     `mapstructure:"ai_model"`
	Feishu       FeishuConfig      `mapstructure:"feishu"`
	SSO          SSOConfig         `mapstructure:"sso"`
}

type ServerConfig struct {
	Addr string `mapstructure:"addr"`
}

type MySQLConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
}

type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	AllowedHeaders []string `mapstructure:"allowed_headers"`
	AllowedMethods []string `mapstructure:"allowed_methods"`
}

type PlatformAPIConfig struct {
	YouTubeAPIKey        string `mapstructure:"youtube_api_key"`
	YouTubeProxyURL      string `mapstructure:"youtube_proxy_url"`
	MetaGraphAPIVersion  string `mapstructure:"meta_graph_api_version"`
	InstagramAccessToken string `mapstructure:"instagram_access_token"`
	InstagramUserID      string `mapstructure:"instagram_user_id"`
	TikTokAccessToken    string `mapstructure:"tiktok_access_token"`
	TikHubAPIKey         string `mapstructure:"tikhub_api_key"`
	SimilarwebAPIKey     string `mapstructure:"similarweb_api_key"`
}

type CollectorConfig struct {
	AgentToken string `mapstructure:"agent_token"`
}

type AIModelConfig struct {
	APIKey string `mapstructure:"api_key"`
}

type FeishuConfig struct {
	ApplicationEnabled bool   `mapstructure:"application_enabled"`
	AppID              string `mapstructure:"app_id"`
	AppSecret          string `mapstructure:"app_secret"`
	APIBaseURL         string `mapstructure:"api_base_url"`
	WebhookEnabled     bool   `mapstructure:"webhook_enabled"`
	WebhookURL         string `mapstructure:"webhook_url"`
	FrontendURL        string `mapstructure:"frontend_url"`
}

type SSOConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	Provider        string `mapstructure:"provider"`
	FrontendURL     string `mapstructure:"frontend_url"`
	RedirectURI     string `mapstructure:"redirect_uri"`
	DefaultRoleCode string `mapstructure:"default_role_code"`
	UACGateway      string `mapstructure:"uac_gateway"`
	UACAppID        string `mapstructure:"uac_app_id"`
	UACLang         string `mapstructure:"uac_lang"`
	UACSource       string `mapstructure:"uac_source"`
}

func loadConfig() (Config, *viper.Viper, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	if file := strings.TrimSpace(os.Getenv("CONFIG_FILE")); file != "" {
		v.SetConfigFile(file)
	} else {
		v.SetConfigName("config")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
	}
	setConfigDefaults(v)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return Config{}, nil, err
	}
	cfg, err := decodeConfig(v)
	if err != nil {
		return Config{}, nil, err
	}
	log.Printf("config loaded from %s", v.ConfigFileUsed())
	return cfg, v, nil
}

func setConfigDefaults(v *viper.Viper) {
	v.SetDefault("server.addr", ":8080")
	v.SetDefault("mysql.username", "root")
	v.SetDefault("mysql.password", "password")
	v.SetDefault("mysql.database", "kol_admin")
	v.SetDefault("cors.allowed_origins", []string{"*"})
	v.SetDefault("cors.allowed_headers", []string{"Authorization", "Content-Type", "X-Requested-With", "X-Collector-Token"})
	v.SetDefault("cors.allowed_methods", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"})
	v.SetDefault("platform_apis.meta_graph_api_version", "v21.0")
	v.SetDefault("collector.agent_token", "")
	v.SetDefault("ai_model.api_key", "")
	v.SetDefault("feishu.api_base_url", "https://open.feishu.cn")
	v.SetDefault("sso.provider", "uac")
	v.SetDefault("sso.default_role_code", "operation")
	v.SetDefault("sso.uac_lang", "zh_CN")
}

func decodeConfig(v *viper.Viper) (Config, error) {
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, err
	}
	cfg = cfg.withDefaults()
	if cfg.SSO.Enabled && !cfg.SSO.ready() {
		return Config{}, errors.New("SSO 已启用，但 provider、frontend_url、redirect_uri、uac_gateway 或 uac_app_id 配置不完整")
	}
	return cfg, nil
}

func (cfg Config) withDefaults() Config {
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":8080"
	}
	if cfg.MySQL.Username == "" {
		cfg.MySQL.Username = "root"
	}
	if cfg.MySQL.Database == "" {
		cfg.MySQL.Database = "kol_admin"
	}
	if len(cfg.CORS.AllowedOrigins) == 0 {
		cfg.CORS.AllowedOrigins = []string{"*"}
	}
	if len(cfg.CORS.AllowedHeaders) == 0 {
		cfg.CORS.AllowedHeaders = []string{"Authorization", "Content-Type", "X-Requested-With", "X-Collector-Token"}
	}
	if len(cfg.CORS.AllowedMethods) == 0 {
		cfg.CORS.AllowedMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	}
	if cfg.PlatformAPIs.MetaGraphAPIVersion == "" {
		cfg.PlatformAPIs.MetaGraphAPIVersion = "v21.0"
	}
	if strings.TrimSpace(cfg.Feishu.APIBaseURL) == "" {
		cfg.Feishu.APIBaseURL = "https://open.feishu.cn"
	}
	if strings.TrimSpace(cfg.SSO.Provider) == "" {
		cfg.SSO.Provider = "uac"
	}
	if strings.TrimSpace(cfg.SSO.DefaultRoleCode) == "" {
		cfg.SSO.DefaultRoleCode = "operation"
	}
	if strings.TrimSpace(cfg.SSO.UACLang) == "" {
		cfg.SSO.UACLang = "zh_CN"
	}
	return cfg
}

func watchConfig(v *viper.Viper, onChange func(Config)) {
	v.OnConfigChange(func(e fsnotify.Event) {
		log.Printf("config file changed: %s", e.Name)
		cfg, err := decodeConfig(v)
		if err != nil {
			log.Printf("decode config failed: %v", err)
			return
		}
		onChange(cfg)
	})
	v.WatchConfig()
}

func openDB(cfg MySQLConfig) (*sql.DB, error) {
	db, err := sql.Open("mysql", mysqlDSN(cfg))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func mysqlDSN(cfg MySQLConfig) string {
	params := url.Values{}
	params.Set("charset", "utf8mb4")
	params.Set("loc", "Local")
	params.Set("parseTime", "true")
	return fmt.Sprintf(
		"%s:%s@tcp(127.0.0.1:3306)/%s?%s",
		cfg.Username,
		cfg.Password,
		cfg.Database,
		params.Encode(),
	)
}
