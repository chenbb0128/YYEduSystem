package config

import (
	"strings"
	"testing"
	"time"
)

func TestDatabaseValidation(t *testing.T) {
	cfg := validConfig()
	cfg.Database.Enabled = true
	cfg.Database.DSN = ""

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "database.dsn") {
		t.Fatalf("Validate() error = %v, want database.dsn error", err)
	}
}

func TestDatabaseDisabledDoesNotRequireDSN(t *testing.T) {
	cfg := validConfig()
	cfg.Database.Enabled = false
	cfg.Database.DSN = ""

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestRedisValidation(t *testing.T) {
	cfg := validConfig()
	cfg.Redis.Enabled = true
	cfg.Redis.Addr = "redis-without-port"

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "redis.addr") {
		t.Fatalf("Validate() error = %v, want redis.addr error", err)
	}
}

func TestWorkerRequiresRedis(t *testing.T) {
	cfg := validConfig()
	cfg.Redis.Enabled = false
	cfg.Worker.Enabled = true

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "requires redis.enabled") {
		t.Fatalf("Validate() error = %v, want worker requires redis error", err)
	}
}

func TestWorkerQueuesValidation(t *testing.T) {
	cfg := validConfig()
	cfg.Redis.Enabled = true
	cfg.Worker.Enabled = true
	cfg.Worker.Queues = map[string]int{"default": 0}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "worker.queues") {
		t.Fatalf("Validate() error = %v, want worker.queues error", err)
	}
}

func TestMetricsValidation(t *testing.T) {
	cfg := validConfig()
	cfg.Observability.Metrics.Enabled = true
	cfg.Observability.Metrics.Path = "metrics"

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "observability.metrics.path") {
		t.Fatalf("Validate() error = %v, want metrics path error", err)
	}
}

func TestTracingValidation(t *testing.T) {
	cfg := validConfig()
	cfg.Observability.Tracing.Enabled = true
	cfg.Observability.Tracing.Exporter = "unknown"

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "observability.tracing.exporter") {
		t.Fatalf("Validate() error = %v, want tracing exporter error", err)
	}
}

func TestTracingSampleRatioValidation(t *testing.T) {
	cfg := validConfig()
	cfg.Observability.Tracing.Enabled = true
	cfg.Observability.Tracing.SampleRatio = 1.1

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "sample_ratio") {
		t.Fatalf("Validate() error = %v, want sample ratio error", err)
	}
}

func TestSanitizedSummaryRedactsSecrets(t *testing.T) {
	cfg := validConfig()
	cfg.Database.DSN = "user:secret@tcp(127.0.0.1:3306)/tuoguan_system"
	cfg.Redis.Password = "redis-secret"
	cfg.OCR.SecretID = "ocr-id"
	cfg.OCR.SecretKey = "ocr-secret"

	summary := cfg.SanitizedSummary()
	if summary["database_dsn"] == cfg.Database.DSN {
		t.Fatal("SanitizedSummary leaked database dsn")
	}
	if summary["redis_password"] == cfg.Redis.Password {
		t.Fatal("SanitizedSummary leaked redis password")
	}
	if summary["ocr_secret_id_configured"] != true || summary["ocr_secret_key_configured"] != true {
		t.Fatalf("SanitizedSummary did not report OCR secret configuration: %#v", summary)
	}
	if _, exists := summary["ocr_secret_key"]; exists {
		t.Fatal("SanitizedSummary exposed OCR secret key")
	}
}

func TestOCRValidationRequiresTencentCredentialsWhenEnabled(t *testing.T) {
	cfg := validConfig()
	cfg.OCR.Enabled = true
	cfg.OCR.Provider = "tencent"
	cfg.OCR.SecretID = ""
	cfg.OCR.SecretKey = "secret-key"
	cfg.OCR.Region = "ap-guangzhou"
	cfg.OCR.Endpoint = "https://ocr.tencentcloudapi.com/"
	cfg.OCR.Timeout = time.Second
	cfg.OCR.Action = "GeneralHandwritingOCR"
	cfg.OCR.MaxImageBytes = 1024

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "ocr.secret_id") {
		t.Fatalf("Validate() error = %v, want ocr.secret_id error", err)
	}
}

func TestOCRValidationAllowsTencentWhenConfigured(t *testing.T) {
	cfg := validConfig()
	cfg.OCR.Enabled = true
	cfg.OCR.Provider = "tencent"
	cfg.OCR.SecretID = "secret-id"
	cfg.OCR.SecretKey = "secret-key"
	cfg.OCR.Region = "ap-guangzhou"
	cfg.OCR.Endpoint = "https://ocr.tencentcloudapi.com/"
	cfg.OCR.Timeout = time.Second
	cfg.OCR.Action = "GeneralHandwritingOCR"
	cfg.OCR.MaxImageBytes = 1024

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestOCRValidationAllowsRapidOCRWithoutCloudCredentials(t *testing.T) {
	cfg := validConfig()
	cfg.OCR.Enabled = true
	cfg.OCR.Provider = "rapidocr"
	cfg.OCR.Endpoint = "http://127.0.0.1:9009/ocr"
	cfg.OCR.Timeout = time.Second
	cfg.OCR.MaxImageBytes = 1024

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestProductionRejectsBootstrapAdmin(t *testing.T) {
	cfg := validConfig()
	cfg.App.Env = "production"
	cfg.Auth.Secret = strings.Repeat("s", 40)
	cfg.Auth.BootstrapAdminEnabled = true

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "bootstrap_admin_enabled") {
		t.Fatalf("Validate() error = %v, want bootstrap admin error", err)
	}
}

func TestProductionRejectsDefaultAuthSecret(t *testing.T) {
	cfg := validConfig()
	cfg.App.Env = "prod"
	cfg.Auth.Secret = DefaultAuthSecret
	cfg.Auth.BootstrapAdminEnabled = false

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "auth.secret") {
		t.Fatalf("Validate() error = %v, want auth secret error", err)
	}
}

func TestProductionRequiresDatabase(t *testing.T) {
	cfg := validConfig()
	cfg.App.Env = "production"
	cfg.Auth.Secret = strings.Repeat("s", 40)
	cfg.Auth.BootstrapAdminEnabled = false
	cfg.Database.Enabled = false

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "database.enabled") {
		t.Fatalf("Validate() error = %v, want production database error", err)
	}
}

func TestProductionWechatRequiresWorker(t *testing.T) {
	cfg := validConfig()
	cfg.App.Env = "production"
	cfg.Auth.Secret = strings.Repeat("s", 40)
	cfg.Auth.BootstrapAdminEnabled = false
	cfg.WeChat.Enabled = true
	cfg.WeChat.AppID = "wx-app"
	cfg.WeChat.Secret = "wx-secret"
	cfg.WeChat.Endpoint = "https://api.weixin.qq.com/sns/jscode2session"
	cfg.WeChat.Timeout = time.Second
	cfg.Worker.Enabled = false

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "worker.enabled") {
		t.Fatalf("Validate() error = %v, want production worker error", err)
	}
}

func TestLocalBootstrapAdminCanBeConfigured(t *testing.T) {
	cfg := validConfig()
	cfg.Auth.BootstrapAdminEnabled = true
	cfg.Auth.BootstrapAdminUsername = "operator"
	cfg.Auth.BootstrapAdminPassword = "local-pass"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestWechatTemplateDataSupportsPerKindFieldMapping(t *testing.T) {
	at := time.Date(2026, 9, 2, 16, 30, 0, 0, time.UTC)
	cfg := WeChatConfig{SubscribeFields: map[string]map[string]string{
		"pickup": {"title": "thing7", "content": "phrase2", "time": "time5"},
	}}
	data := cfg.TemplateDataForKind("pickup", "接送安排", "王老师已接到孩子", at)
	if data["thing7"] != "接送安排" || data["phrase2"] != "王老师已接到孩子" || data["time5"] != "2026-09-02 16:30" {
		t.Fatalf("mapped template data = %#v", data)
	}
	if _, exists := data["thing1"]; exists {
		t.Fatalf("mapped template data unexpectedly contains default fields: %#v", data)
	}
}

func validConfig() Config {
	return Config{
		App: AppConfig{Name: "tuoguan-system", Env: "local"},
		HTTP: HTTPConfig{
			Addr:              ":8080",
			ReadHeaderTimeout: time.Second,
			ReadTimeout:       time.Second,
			WriteTimeout:      time.Second,
			IdleTimeout:       time.Second,
			ShutdownTimeout:   time.Second,
			MaxHeaderBytes:    1,
			MaxBodyBytes:      1,
			CORS: CORSConfig{
				AllowedOrigins: []string{"http://localhost:3000"},
				AllowedMethods: []string{"GET"},
				AllowedHeaders: []string{"Content-Type"},
			},
		},
		Database: DatabaseConfig{
			Enabled:         true,
			Driver:          "mysql",
			DSN:             "user:pass@tcp(127.0.0.1:3306)/tuoguan_system",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: time.Minute,
			ConnMaxIdleTime: time.Minute,
			PingTimeout:     time.Second,
		},
		Redis: RedisConfig{
			Enabled:      false,
			Addr:         "127.0.0.1:6379",
			DB:           0,
			DialTimeout:  time.Second,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
			PingTimeout:  time.Second,
			PoolSize:     10,
			MinIdleConns: 2,
			KeyPrefix:    "tuoguan-system:test",
		},
		Storage: StorageConfig{
			UploadDir: "data/uploads",
		},
		Worker: WorkerConfig{
			Enabled:         false,
			Concurrency:     10,
			ShutdownTimeout: time.Second,
			Queues:          map[string]int{"default": 1},
		},
		Observability: ObservabilityConfig{
			Metrics: MetricsConfig{
				Enabled:   true,
				Path:      "/metrics",
				Namespace: "tuoguan_system",
			},
			Tracing: TracingConfig{
				Enabled:     false,
				Exporter:    "stdout",
				Endpoint:    "localhost:4317",
				Insecure:    true,
				SampleRatio: 1,
			},
		},
		Log: LogConfig{Level: "info"},
	}
}
