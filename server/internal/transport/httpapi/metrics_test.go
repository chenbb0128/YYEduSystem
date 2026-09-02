package httpapi_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chenbb0128/tuoguan-system-server/internal/config"
	platformmetrics "github.com/chenbb0128/tuoguan-system-server/internal/platform/metrics"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi"
)

func TestRouterMetricsEndpoint(t *testing.T) {
	metrics, err := platformmetrics.New(config.MetricsConfig{
		Enabled:   true,
		Path:      "/metrics",
		Namespace: "tuoguan_system",
	})
	if err != nil {
		t.Fatalf("metrics.New() error = %v", err)
	}

	router, err := httpapi.NewRouter(httpapi.RouterOptions{
		App: config.AppConfig{Name: "tuoguan-system", Env: "test"},
		HTTP: config.HTTPConfig{
			Addr:              ":0",
			ReadHeaderTimeout: time.Second,
			ReadTimeout:       time.Second,
			WriteTimeout:      time.Second,
			IdleTimeout:       time.Second,
			ShutdownTimeout:   time.Second,
			MaxHeaderBytes:    1 << 20,
			MaxBodyBytes:      1 << 20,
			TrustedProxies:    []string{},
			CORS: config.CORSConfig{
				AllowedOrigins: []string{"http://localhost:3000"},
				AllowedMethods: []string{"GET", "OPTIONS"},
				AllowedHeaders: []string{"Content-Type", "X-Request-ID"},
			},
		},
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Metrics: metrics,
	})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	healthRec := httptest.NewRecorder()
	healthReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", healthRec.Code, http.StatusOK)
	}

	metricsRec := httptest.NewRecorder()
	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	router.ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d", metricsRec.Code, http.StatusOK)
	}
	if body := metricsRec.Body.String(); !strings.Contains(body, "tuoguan_system_http_requests_total") {
		t.Fatalf("metrics body does not contain http counter: %s", body)
	}
}
