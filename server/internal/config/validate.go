package config

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
)

var metricNamespacePattern = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)

func (c Config) Validate() error {
	if strings.TrimSpace(c.App.Name) == "" {
		return fmt.Errorf("config app.name is required")
	}

	env := strings.ToLower(strings.TrimSpace(c.App.Env))
	if !slices.Contains([]string{"local", "dev", "test", "staging", "prod", "production"}, env) {
		return fmt.Errorf("config app.env must be one of local, dev, test, staging, prod, production")
	}

	if _, _, err := net.SplitHostPort(c.HTTP.Addr); err != nil {
		return fmt.Errorf("config http.addr must be host:port or :port: %w", err)
	}

	if err := positiveDuration("http.read_header_timeout", c.HTTP.ReadHeaderTimeout); err != nil {
		return err
	}
	if err := positiveDuration("http.read_timeout", c.HTTP.ReadTimeout); err != nil {
		return err
	}
	if err := positiveDuration("http.write_timeout", c.HTTP.WriteTimeout); err != nil {
		return err
	}
	if err := positiveDuration("http.idle_timeout", c.HTTP.IdleTimeout); err != nil {
		return err
	}
	if err := positiveDuration("http.shutdown_timeout", c.HTTP.ShutdownTimeout); err != nil {
		return err
	}

	if c.HTTP.MaxHeaderBytes <= 0 {
		return fmt.Errorf("config http.max_header_bytes must be positive")
	}
	if c.HTTP.MaxBodyBytes <= 0 {
		return fmt.Errorf("config http.max_body_bytes must be positive")
	}

	for _, proxy := range c.HTTP.TrustedProxies {
		if err := validateProxy(proxy); err != nil {
			return fmt.Errorf("config http.trusted_proxies contains invalid value %q: %w", proxy, err)
		}
	}

	if err := c.HTTP.CORS.Validate(env); err != nil {
		return err
	}
	if err := c.Auth.Validate(env); err != nil {
		return err
	}
	if err := c.Database.Validate(); err != nil {
		return err
	}
	if err := c.WeChat.Validate(); err != nil {
		return err
	}
	if err := c.SMS.Validate(env); err != nil {
		return err
	}
	if err := c.OCR.Validate(env); err != nil {
		return err
	}
	if err := c.Storage.Validate(env); err != nil {
		return err
	}
	if err := c.Redis.Validate(); err != nil {
		return err
	}
	if err := c.Worker.Validate(c.Redis); err != nil {
		return err
	}
	if err := c.Observability.Validate(); err != nil {
		return err
	}
	if env == "prod" || env == "production" {
		if !c.Database.Enabled {
			return fmt.Errorf("config database.enabled must be true in production")
		}
		if c.WeChat.Enabled && !c.Worker.Enabled {
			return fmt.Errorf("config worker.enabled must be true when wechat.enabled is true in production")
		}
		if strings.EqualFold(strings.TrimSpace(c.Storage.Provider), "local") || strings.TrimSpace(c.Storage.Provider) == "" {
			return fmt.Errorf("config storage.provider must be s3 in production")
		}
		if strings.TrimSpace(c.Storage.URLSigningSecret) == "" || strings.TrimSpace(c.Storage.URLSigningSecret) == strings.TrimSpace(c.Auth.Secret) || isPlaceholder(c.Storage.URLSigningSecret) {
			return fmt.Errorf("config storage.url_signing_secret must be an independent secret in production")
		}
		if isPlaceholder(c.Auth.Secret) {
			return fmt.Errorf("config auth.secret must not contain a deployment placeholder in production")
		}
		if isPlaceholder(c.Database.DSN) {
			return fmt.Errorf("config database.dsn must not contain a deployment placeholder in production")
		}
		if c.Storage.SignedURLTTL <= 0 {
			return fmt.Errorf("config storage.signed_url_ttl must be positive in production")
		}
		if c.WeChat.Enabled {
			if isPlaceholder(c.WeChat.AppID) || isPlaceholder(c.WeChat.Secret) {
				return fmt.Errorf("config wechat.app_id and wechat.app_secret must be real values in production")
			}
			for _, kind := range []string{"pickup", "meal", "homework", "leave", "summary"} {
				if strings.TrimSpace(c.WeChat.TemplateForKind(kind)) == "" || isPlaceholder(c.WeChat.TemplateForKind(kind)) {
					return fmt.Errorf("config wechat.subscribe_templates.%s is required in production", kind)
				}
			}
		}
	}

	switch strings.ToLower(strings.TrimSpace(c.Log.Level)) {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("config log.level must be one of debug, info, warn, error")
	}

	return nil
}

func (c SMSConfig) Validate(env string) error {
	provider := strings.ToLower(strings.TrimSpace(c.Provider))
	if provider == "" {
		provider = "local"
	}
	switch provider {
	case "local", "tencent":
	default:
		return fmt.Errorf("config sms.provider must be local or tencent")
	}
	if !c.Enabled {
		return nil
	}
	if c.CodeLength < 4 || c.CodeLength > 8 {
		return fmt.Errorf("config sms.code_length must be between 4 and 8")
	}
	if err := positiveDuration("sms.timeout", c.Timeout); err != nil {
		return err
	}
	if err := positiveDuration("sms.code_ttl", c.CodeTTL); err != nil {
		return err
	}
	if err := positiveDuration("sms.resend_interval", c.ResendInterval); err != nil {
		return err
	}
	if c.MaxVerifyAttempts <= 0 {
		return fmt.Errorf("config sms.max_verify_attempts must be positive")
	}
	if provider == "tencent" {
		for field, value := range map[string]string{
			"sms.secret_id":   c.SecretID,
			"sms.secret_key":  c.SecretKey,
			"sms.sdk_app_id":  c.SDKAppID,
			"sms.sign_name":   c.SignName,
			"sms.template_id": c.TemplateID,
			"sms.region":      c.Region,
			"sms.endpoint":    c.Endpoint,
		} {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("config %s is required when sms.provider is tencent", field)
			}
		}
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(c.Endpoint)), "https://") {
			return fmt.Errorf("config sms.endpoint must use https")
		}
	}
	if (env == "prod" || env == "production") && provider == "local" {
		return fmt.Errorf("config sms.provider local is not allowed in production")
	}
	if (env == "prod" || env == "production") && strings.TrimSpace(c.CodeSecret) == "" {
		return fmt.Errorf("config sms.code_secret is required in production when sms is enabled")
	}
	return nil
}

func (c OCRConfig) Validate(env string) error {
	provider := strings.ToLower(strings.TrimSpace(c.Provider))
	if provider == "" {
		provider = "rapidocr"
	}
	switch provider {
	case "rapidocr", "tencent":
	default:
		return fmt.Errorf("config ocr.provider must be rapidocr or tencent")
	}
	if !c.Enabled {
		return nil
	}
	if strings.TrimSpace(c.Endpoint) == "" {
		return fmt.Errorf("config ocr.endpoint is required when ocr.enabled is true")
	}
	endpoint, err := url.Parse(strings.TrimSpace(c.Endpoint))
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return fmt.Errorf("config ocr.endpoint must be a valid URL")
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return fmt.Errorf("config ocr.endpoint must use http or https")
	}
	if provider == "tencent" {
		for field, value := range map[string]string{
			"ocr.secret_id":  c.SecretID,
			"ocr.secret_key": c.SecretKey,
			"ocr.region":     c.Region,
			"ocr.action":     c.Action,
		} {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("config %s is required when ocr.provider is tencent", field)
			}
		}
		if endpoint.Scheme != "https" {
			return fmt.Errorf("config ocr.endpoint must use https when ocr.provider is tencent")
		}
	}
	if err := positiveDuration("ocr.timeout", c.Timeout); err != nil {
		return err
	}
	if c.MaxImageBytes <= 0 {
		return fmt.Errorf("config ocr.max_image_bytes must be positive")
	}
	if provider == "tencent" {
		switch strings.TrimSpace(c.Action) {
		case "GeneralHandwritingOCR":
		default:
			return fmt.Errorf("config ocr.action currently supports GeneralHandwritingOCR for tencent")
		}
	}
	if env == "prod" || env == "production" {
		if provider == "tencent" && (isPlaceholder(c.SecretID) || isPlaceholder(c.SecretKey)) {
			return fmt.Errorf("config ocr credentials must be real values in production")
		}
	}
	return nil
}

func (c StorageConfig) Validate(env string) error {
	provider := strings.ToLower(strings.TrimSpace(c.Provider))
	if provider == "" {
		provider = "local"
	}
	switch provider {
	case "local":
		if strings.TrimSpace(c.UploadDir) == "" {
			return fmt.Errorf("config storage.upload_dir is required for local provider")
		}
	case "s3":
		if strings.TrimSpace(c.Endpoint) == "" {
			return fmt.Errorf("config storage.endpoint is required for s3 provider")
		}
		if strings.TrimSpace(c.Bucket) == "" {
			return fmt.Errorf("config storage.bucket is required for s3 provider")
		}
		if strings.TrimSpace(c.Region) == "" {
			return fmt.Errorf("config storage.region is required for s3 provider")
		}
		if strings.TrimSpace(c.AccessKey) == "" || strings.TrimSpace(c.SecretKey) == "" {
			return fmt.Errorf("config storage.access_key and storage.secret_key are required for s3 provider")
		}
		if (env == "prod" || env == "production") && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(c.Endpoint)), "https://") {
			return fmt.Errorf("config storage.endpoint must use https in production")
		}
	default:
		return fmt.Errorf("config storage.provider must be local or s3")
	}
	if c.SignedURLTTL < 0 {
		return fmt.Errorf("config storage.signed_url_ttl must not be negative")
	}
	if c.MaxFileBytes < 0 {
		return fmt.Errorf("config storage.max_file_bytes must not be negative")
	}
	if c.RetentionDays < 0 {
		return fmt.Errorf("config storage.retention_days must not be negative")
	}
	if env == "prod" || env == "production" {
		if isPlaceholder(c.Endpoint) || isPlaceholder(c.Bucket) || isPlaceholder(c.AccessKey) || isPlaceholder(c.SecretKey) {
			return fmt.Errorf("config storage endpoint, bucket and credentials must be real values in production")
		}
	}
	return nil
}

func (c AuthConfig) Validate(env string) error {
	if c.BootstrapAdminEnabled {
		if env == "prod" || env == "production" {
			return fmt.Errorf("config auth.bootstrap_admin_enabled must be false in production")
		}
		if len(strings.TrimSpace(c.BootstrapAdminUsername)) < 3 {
			return fmt.Errorf("config auth.bootstrap_admin_username must contain at least 3 characters")
		}
		if len(c.BootstrapAdminPassword) < 6 {
			return fmt.Errorf("config auth.bootstrap_admin_password must contain at least 6 characters")
		}
	}
	if c.BootstrapPlatformAdminEnabled {
		if env == "prod" || env == "production" {
			return fmt.Errorf("config auth.bootstrap_platform_admin_enabled must be false in production")
		}
		if len(strings.TrimSpace(c.BootstrapPlatformAdminUsername)) < 3 {
			return fmt.Errorf("config auth.bootstrap_platform_admin_username must contain at least 3 characters")
		}
		if len(c.BootstrapPlatformAdminPassword) < 6 {
			return fmt.Errorf("config auth.bootstrap_platform_admin_password must contain at least 6 characters")
		}
	}
	if env == "prod" || env == "production" {
		if len(strings.TrimSpace(c.Secret)) < 32 || strings.TrimSpace(c.Secret) == DefaultAuthSecret {
			return fmt.Errorf("config auth.secret must be a non-default secret of at least 32 characters in production")
		}
	}
	return nil
}

func (c WeChatConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if strings.TrimSpace(c.AppID) == "" {
		return fmt.Errorf("config wechat.app_id is required when wechat.enabled is true")
	}
	if strings.TrimSpace(c.Secret) == "" {
		return fmt.Errorf("config wechat.app_secret is required when wechat.enabled is true")
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(c.Endpoint)), "https://") {
		return fmt.Errorf("config wechat.endpoint must use https")
	}
	return positiveDuration("wechat.timeout", c.Timeout)
}

func (c CORSConfig) Validate(env string) error {
	if len(c.AllowedOrigins) == 0 {
		return fmt.Errorf("config http.cors.allowed_origins must not be empty")
	}
	if len(c.AllowedMethods) == 0 {
		return fmt.Errorf("config http.cors.allowed_methods must not be empty")
	}
	if len(c.AllowedHeaders) == 0 {
		return fmt.Errorf("config http.cors.allowed_headers must not be empty")
	}

	hasWildcardOrigin := slices.Contains(c.AllowedOrigins, "*")
	if c.AllowCredentials && hasWildcardOrigin {
		return fmt.Errorf("config http.cors cannot use wildcard origin with credentials")
	}
	if (env == "prod" || env == "production") && hasWildcardOrigin {
		return fmt.Errorf("config http.cors.allowed_origins cannot contain wildcard in production")
	}

	for _, method := range c.AllowedMethods {
		switch strings.ToUpper(method) {
		case "GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD":
		default:
			return fmt.Errorf("config http.cors.allowed_methods contains unsupported method %q", method)
		}
	}

	return nil
}

func (c DatabaseConfig) Validate() error {
	if strings.TrimSpace(c.Driver) == "" {
		return fmt.Errorf("config database.driver is required")
	}
	if c.Driver != "mysql" {
		return fmt.Errorf("config database.driver only supports mysql in this template")
	}
	if !c.Enabled {
		return nil
	}
	if strings.TrimSpace(c.DSN) == "" {
		return fmt.Errorf("config database.dsn is required when database.enabled is true")
	}
	if c.MaxOpenConns <= 0 {
		return fmt.Errorf("config database.max_open_conns must be positive")
	}
	if c.MaxIdleConns < 0 {
		return fmt.Errorf("config database.max_idle_conns must not be negative")
	}
	if c.MaxIdleConns > c.MaxOpenConns {
		return fmt.Errorf("config database.max_idle_conns must not exceed database.max_open_conns")
	}
	if err := positiveDuration("database.conn_max_lifetime", c.ConnMaxLifetime); err != nil {
		return err
	}
	if err := positiveDuration("database.conn_max_idle_time", c.ConnMaxIdleTime); err != nil {
		return err
	}
	if err := positiveDuration("database.ping_timeout", c.PingTimeout); err != nil {
		return err
	}
	return nil
}

func (c RedisConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if strings.TrimSpace(c.Addr) == "" {
		return fmt.Errorf("config redis.addr is required when redis.enabled is true")
	}
	if _, _, err := net.SplitHostPort(c.Addr); err != nil {
		return fmt.Errorf("config redis.addr must be host:port: %w", err)
	}
	if c.DB < 0 {
		return fmt.Errorf("config redis.db must not be negative")
	}
	if err := positiveDuration("redis.dial_timeout", c.DialTimeout); err != nil {
		return err
	}
	if err := positiveDuration("redis.read_timeout", c.ReadTimeout); err != nil {
		return err
	}
	if err := positiveDuration("redis.write_timeout", c.WriteTimeout); err != nil {
		return err
	}
	if err := positiveDuration("redis.ping_timeout", c.PingTimeout); err != nil {
		return err
	}
	if c.PoolSize <= 0 {
		return fmt.Errorf("config redis.pool_size must be positive")
	}
	if c.MinIdleConns < 0 {
		return fmt.Errorf("config redis.min_idle_conns must not be negative")
	}
	if c.MinIdleConns > c.PoolSize {
		return fmt.Errorf("config redis.min_idle_conns must not exceed redis.pool_size")
	}
	if strings.TrimSpace(c.KeyPrefix) == "" {
		return fmt.Errorf("config redis.key_prefix is required when redis.enabled is true")
	}
	return nil
}

func (c WorkerConfig) Validate(redis RedisConfig) error {
	if !c.Enabled {
		return nil
	}
	if !redis.Enabled {
		return fmt.Errorf("config worker.enabled requires redis.enabled to be true")
	}
	if c.Concurrency <= 0 {
		return fmt.Errorf("config worker.concurrency must be positive")
	}
	if err := positiveDuration("worker.shutdown_timeout", c.ShutdownTimeout); err != nil {
		return err
	}
	if c.ScheduleGenerationInterval != 0 {
		if err := positiveDuration("worker.schedule_generation_interval", c.ScheduleGenerationInterval); err != nil {
			return err
		}
	}
	if len(c.Queues) == 0 {
		return fmt.Errorf("config worker.queues must not be empty when worker.enabled is true")
	}
	for name, priority := range c.Queues {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("config worker.queues contains an empty queue name")
		}
		if priority <= 0 {
			return fmt.Errorf("config worker.queues[%s] priority must be positive", name)
		}
	}
	return nil
}

func (c ObservabilityConfig) Validate() error {
	if err := c.Metrics.Validate(); err != nil {
		return err
	}
	if err := c.Tracing.Validate(); err != nil {
		return err
	}
	return nil
}

func (c MetricsConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	path := strings.TrimSpace(c.Path)
	if path == "" || !strings.HasPrefix(path, "/") {
		return fmt.Errorf("config observability.metrics.path must start with /")
	}
	if strings.ContainsAny(path, " \t\r\n") {
		return fmt.Errorf("config observability.metrics.path must not contain whitespace")
	}
	if !metricNamespacePattern.MatchString(strings.TrimSpace(c.Namespace)) {
		return fmt.Errorf("config observability.metrics.namespace must be a valid prometheus namespace")
	}
	return nil
}

func (c TracingConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(c.Exporter)) {
	case "stdout", "otlp":
	default:
		return fmt.Errorf("config observability.tracing.exporter must be one of stdout, otlp")
	}
	if strings.EqualFold(strings.TrimSpace(c.Exporter), "otlp") && strings.TrimSpace(c.Endpoint) == "" {
		return fmt.Errorf("config observability.tracing.endpoint is required when exporter is otlp")
	}
	if c.SampleRatio < 0 || c.SampleRatio > 1 {
		return fmt.Errorf("config observability.tracing.sample_ratio must be between 0 and 1")
	}
	return nil
}

func positiveDuration(name string, value time.Duration) error {
	if value <= 0 {
		return fmt.Errorf("config %s must be positive", name)
	}
	return nil
}

func isPlaceholder(value string) bool {
	value = strings.ToUpper(strings.TrimSpace(value))
	return strings.Contains(value, "REPLACE_WITH") || strings.Contains(value, "CHANGE_ME") || strings.Contains(value, "EXAMPLE.COM")
}

func validateProxy(value string) error {
	if _, err := netip.ParsePrefix(value); err == nil {
		return nil
	}
	if _, err := netip.ParseAddr(value); err == nil {
		return nil
	}
	return fmt.Errorf("expected IP address or CIDR prefix")
}
