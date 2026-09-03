package config

import (
	"strings"
	"time"
)

type Config struct {
	App           AppConfig           `mapstructure:"app"`
	HTTP          HTTPConfig          `mapstructure:"http"`
	Auth          AuthConfig          `mapstructure:"auth"`
	WeChat        WeChatConfig        `mapstructure:"wechat"`
	SMS           SMSConfig           `mapstructure:"sms"`
	Storage       StorageConfig       `mapstructure:"storage"`
	Database      DatabaseConfig      `mapstructure:"database"`
	Redis         RedisConfig         `mapstructure:"redis"`
	Worker        WorkerConfig        `mapstructure:"worker"`
	Observability ObservabilityConfig `mapstructure:"observability"`
	Log           LogConfig           `mapstructure:"log"`
}

type AuthConfig struct {
	Secret                         string        `mapstructure:"secret"`
	AccessTTL                      time.Duration `mapstructure:"access_ttl"`
	RefreshTTL                     time.Duration `mapstructure:"refresh_ttl"`
	BootstrapAdminEnabled          bool          `mapstructure:"bootstrap_admin_enabled"`
	BootstrapAdminUsername         string        `mapstructure:"bootstrap_admin_username"`
	BootstrapAdminPassword         string        `mapstructure:"bootstrap_admin_password"`
	BootstrapPlatformAdminEnabled  bool          `mapstructure:"bootstrap_platform_admin_enabled"`
	BootstrapPlatformAdminUsername string        `mapstructure:"bootstrap_platform_admin_username"`
	BootstrapPlatformAdminPassword string        `mapstructure:"bootstrap_platform_admin_password"`
}

const DefaultAuthSecret = "tuoguan-system-local-auth-secret-change-me-2026"

type WeChatConfig struct {
	Enabled             bool                         `mapstructure:"enabled"`
	AppID               string                       `mapstructure:"app_id"`
	Secret              string                       `mapstructure:"app_secret"`
	Endpoint            string                       `mapstructure:"endpoint"`
	Timeout             time.Duration                `mapstructure:"timeout"`
	SubscribeTemplateID string                       `mapstructure:"subscribe_template_id"`
	SubscribePage       string                       `mapstructure:"subscribe_page"`
	SubscribeTemplates  map[string]string            `mapstructure:"subscribe_templates"`
	SubscribeFields     map[string]map[string]string `mapstructure:"subscribe_template_fields"`
}

// SMSConfig controls phone verification-code delivery. The local provider is
// intended for development only; production should use a real provider.
type SMSConfig struct {
	Enabled           bool          `mapstructure:"enabled"`
	Provider          string        `mapstructure:"provider"`
	SecretID          string        `mapstructure:"secret_id"`
	SecretKey         string        `mapstructure:"secret_key"`
	SDKAppID          string        `mapstructure:"sdk_app_id"`
	SignName          string        `mapstructure:"sign_name"`
	TemplateID        string        `mapstructure:"template_id"`
	Region            string        `mapstructure:"region"`
	Endpoint          string        `mapstructure:"endpoint"`
	Timeout           time.Duration `mapstructure:"timeout"`
	CodeSecret        string        `mapstructure:"code_secret"`
	CodeLength        int           `mapstructure:"code_length"`
	CodeTTL           time.Duration `mapstructure:"code_ttl"`
	ResendInterval    time.Duration `mapstructure:"resend_interval"`
	MaxVerifyAttempts int           `mapstructure:"max_verify_attempts"`
}

func (c WeChatConfig) TemplateForKind(kind string) string {
	if value := strings.TrimSpace(c.SubscribeTemplates[strings.TrimSpace(kind)]); value != "" {
		return value
	}
	return strings.TrimSpace(c.SubscribeTemplateID)
}

func (c WeChatConfig) HasSubscribeTemplates() bool {
	if strings.TrimSpace(c.SubscribeTemplateID) != "" {
		return true
	}
	for _, value := range c.SubscribeTemplates {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

// TemplateDataForKind maps business values to the keyword fields of the
// configured WeChat template. Different templates may use different keyword
// IDs, so production deployments can override the local defaults per kind.
func (c WeChatConfig) TemplateDataForKind(kind, title, content string, at time.Time) map[string]string {
	fields := c.SubscribeFields[strings.TrimSpace(kind)]
	if len(fields) == 0 {
		return map[string]string{
			"thing1": title,
			"thing2": content,
			"time3":  at.Format("2006-01-02 15:04"),
		}
	}
	values := make(map[string]string, len(fields))
	for valueName, fieldName := range fields {
		fieldName = strings.TrimSpace(fieldName)
		if fieldName == "" {
			continue
		}
		switch strings.TrimSpace(valueName) {
		case "title":
			values[fieldName] = title
		case "content":
			values[fieldName] = content
		case "time":
			values[fieldName] = at.Format("2006-01-02 15:04")
		}
	}
	return values
}

type StorageConfig struct {
	Provider         string        `mapstructure:"provider"`
	UploadDir        string        `mapstructure:"upload_dir"`
	URLSigningSecret string        `mapstructure:"url_signing_secret"`
	Endpoint         string        `mapstructure:"endpoint"`
	Bucket           string        `mapstructure:"bucket"`
	Region           string        `mapstructure:"region"`
	AccessKey        string        `mapstructure:"access_key"`
	SecretKey        string        `mapstructure:"secret_key"`
	PathStyle        bool          `mapstructure:"path_style"`
	SignedURLTTL     time.Duration `mapstructure:"signed_url_ttl"`
	MaxFileBytes     int64         `mapstructure:"max_file_bytes"`
	RetentionDays    int           `mapstructure:"retention_days"`
}

type AppConfig struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
}

type HTTPConfig struct {
	Addr              string        `mapstructure:"addr"`
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"`
	IdleTimeout       time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout   time.Duration `mapstructure:"shutdown_timeout"`
	MaxHeaderBytes    int           `mapstructure:"max_header_bytes"`
	MaxBodyBytes      int64         `mapstructure:"max_body_bytes"`
	TrustedProxies    []string      `mapstructure:"trusted_proxies"`
	CORS              CORSConfig    `mapstructure:"cors"`
}

type CORSConfig struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
}

type DatabaseConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	Driver          string        `mapstructure:"driver"`
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
	PingTimeout     time.Duration `mapstructure:"ping_timeout"`
}

type RedisConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	Addr         string        `mapstructure:"addr"`
	Username     string        `mapstructure:"username"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	PingTimeout  time.Duration `mapstructure:"ping_timeout"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	KeyPrefix    string        `mapstructure:"key_prefix"`
}

type WorkerConfig struct {
	Enabled                    bool           `mapstructure:"enabled"`
	Concurrency                int            `mapstructure:"concurrency"`
	ShutdownTimeout            time.Duration  `mapstructure:"shutdown_timeout"`
	Queues                     map[string]int `mapstructure:"queues"`
	NotificationPollInterval   time.Duration  `mapstructure:"notification_poll_interval"`
	NotificationLease          time.Duration  `mapstructure:"notification_lease"`
	NotificationMaxAttempts    int            `mapstructure:"notification_max_attempts"`
	ScheduleGenerationInterval time.Duration  `mapstructure:"schedule_generation_interval"`
}

type ObservabilityConfig struct {
	Metrics MetricsConfig `mapstructure:"metrics"`
	Tracing TracingConfig `mapstructure:"tracing"`
}

type MetricsConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Path      string `mapstructure:"path"`
	Namespace string `mapstructure:"namespace"`
}

type TracingConfig struct {
	Enabled     bool    `mapstructure:"enabled"`
	Exporter    string  `mapstructure:"exporter"`
	Endpoint    string  `mapstructure:"endpoint"`
	Insecure    bool    `mapstructure:"insecure"`
	SampleRatio float64 `mapstructure:"sample_ratio"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
}

func (c Config) SanitizedSummary() map[string]any {
	return map[string]any{
		"app_name":                            c.App.Name,
		"env":                                 c.App.Env,
		"http_addr":                           c.HTTP.Addr,
		"auth_secret_configured":              strings.TrimSpace(c.Auth.Secret) != "",
		"auth_access_ttl":                     c.Auth.AccessTTL.String(),
		"auth_refresh_ttl":                    c.Auth.RefreshTTL.String(),
		"bootstrap_admin_enabled":             c.Auth.BootstrapAdminEnabled,
		"bootstrap_admin_username":            c.Auth.BootstrapAdminUsername,
		"bootstrap_platform_admin_enabled":    c.Auth.BootstrapPlatformAdminEnabled,
		"bootstrap_platform_admin_username":   c.Auth.BootstrapPlatformAdminUsername,
		"wechat_enabled":                      c.WeChat.Enabled,
		"wechat_app_id_configured":            strings.TrimSpace(c.WeChat.AppID) != "",
		"wechat_secret_configured":            strings.TrimSpace(c.WeChat.Secret) != "",
		"wechat_subscribe_configured":         c.WeChat.HasSubscribeTemplates(),
		"wechat_subscribe_template_count":     subscribeTemplateCount(c.WeChat.SubscribeTemplates, c.WeChat.SubscribeTemplateID),
		"sms_enabled":                         c.SMS.Enabled,
		"sms_provider":                        c.SMS.Provider,
		"sms_secret_id_configured":            strings.TrimSpace(c.SMS.SecretID) != "",
		"sms_secret_key_configured":           strings.TrimSpace(c.SMS.SecretKey) != "",
		"sms_sdk_app_id_configured":           strings.TrimSpace(c.SMS.SDKAppID) != "",
		"sms_sign_name_configured":            strings.TrimSpace(c.SMS.SignName) != "",
		"sms_template_id_configured":          strings.TrimSpace(c.SMS.TemplateID) != "",
		"sms_code_secret_configured":          strings.TrimSpace(c.SMS.CodeSecret) != "",
		"sms_code_length":                     c.SMS.CodeLength,
		"sms_code_ttl":                        c.SMS.CodeTTL.String(),
		"sms_resend_interval":                 c.SMS.ResendInterval.String(),
		"sms_max_verify_attempts":             c.SMS.MaxVerifyAttempts,
		"storage_provider":                    c.Storage.Provider,
		"storage_upload_dir":                  c.Storage.UploadDir,
		"storage_endpoint":                    c.Storage.Endpoint,
		"storage_bucket":                      c.Storage.Bucket,
		"storage_region":                      c.Storage.Region,
		"storage_access_key_configured":       strings.TrimSpace(c.Storage.AccessKey) != "",
		"storage_secret_key_configured":       strings.TrimSpace(c.Storage.SecretKey) != "",
		"storage_path_style":                  c.Storage.PathStyle,
		"storage_signed_url_ttl":              c.Storage.SignedURLTTL.String(),
		"storage_max_file_bytes":              c.Storage.MaxFileBytes,
		"storage_retention_days":              c.Storage.RetentionDays,
		"storage_url_signing_configured":      strings.TrimSpace(c.Storage.URLSigningSecret) != "",
		"read_header":                         c.HTTP.ReadHeaderTimeout.String(),
		"read":                                c.HTTP.ReadTimeout.String(),
		"write":                               c.HTTP.WriteTimeout.String(),
		"idle":                                c.HTTP.IdleTimeout.String(),
		"shutdown":                            c.HTTP.ShutdownTimeout.String(),
		"max_header_bytes":                    c.HTTP.MaxHeaderBytes,
		"max_body_bytes":                      c.HTTP.MaxBodyBytes,
		"trusted_proxies":                     len(c.HTTP.TrustedProxies),
		"cors_origins":                        len(c.HTTP.CORS.AllowedOrigins),
		"cors_credentials":                    c.HTTP.CORS.AllowCredentials,
		"database_enabled":                    c.Database.Enabled,
		"database_driver":                     c.Database.Driver,
		"database_dsn":                        redactSecret(c.Database.DSN),
		"database_max_open":                   c.Database.MaxOpenConns,
		"database_max_idle":                   c.Database.MaxIdleConns,
		"redis_enabled":                       c.Redis.Enabled,
		"redis_addr":                          c.Redis.Addr,
		"redis_username":                      c.Redis.Username,
		"redis_password":                      redactSecret(c.Redis.Password),
		"redis_db":                            c.Redis.DB,
		"redis_pool_size":                     c.Redis.PoolSize,
		"redis_key_prefix":                    c.Redis.KeyPrefix,
		"worker_enabled":                      c.Worker.Enabled,
		"worker_concurrency":                  c.Worker.Concurrency,
		"worker_queues":                       len(c.Worker.Queues),
		"worker_notification_poll_interval":   c.Worker.NotificationPollInterval.String(),
		"worker_notification_lease":           c.Worker.NotificationLease.String(),
		"worker_notification_max_attempts":    c.Worker.NotificationMaxAttempts,
		"worker_schedule_generation_interval": c.Worker.ScheduleGenerationInterval.String(),
		"metrics_enabled":                     c.Observability.Metrics.Enabled,
		"metrics_path":                        c.Observability.Metrics.Path,
		"metrics_namespace":                   c.Observability.Metrics.Namespace,
		"tracing_enabled":                     c.Observability.Tracing.Enabled,
		"tracing_exporter":                    c.Observability.Tracing.Exporter,
		"tracing_endpoint_configured":         c.Observability.Tracing.Endpoint != "",
		"tracing_insecure":                    c.Observability.Tracing.Insecure,
		"tracing_sample_ratio":                c.Observability.Tracing.SampleRatio,
		"log_level":                           c.Log.Level,
	}
}

func subscribeTemplateCount(values map[string]string, legacy string) int {
	seen := map[string]struct{}{}
	if strings.TrimSpace(legacy) != "" {
		seen[strings.TrimSpace(legacy)] = struct{}{}
	}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			seen[strings.TrimSpace(value)] = struct{}{}
		}
	}
	return len(seen)
}

func redactSecret(value string) string {
	if value == "" {
		return ""
	}
	return "<redacted>"
}
