package verification

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chenbb0128/tuoguan-system-server/internal/config"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/sms"
)

func TestServiceIssuesAndConsumesRandomLocalCode(t *testing.T) {
	service, err := NewService(NewMemoryStore(), sms.LocalSender{}, config.SMSConfig{
		CodeSecret:        "test-code-secret",
		CodeLength:        6,
		CodeTTL:           5 * time.Minute,
		ResendInterval:    time.Minute,
		MaxVerifyAttempts: 5,
	}, "fallback-secret")
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Issue(context.Background(), "13800000000")
	if err != nil {
		t.Fatal(err)
	}
	if result.DebugCode == "" || len(result.DebugCode) != 6 {
		t.Fatalf("local issue result = %+v", result)
	}
	if err := service.Verify(context.Background(), result.Phone, result.DebugCode); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if err := service.Verify(context.Background(), result.Phone, result.DebugCode); !errors.Is(err, ErrCodeExpired) {
		t.Fatalf("second Verify() error = %v, want expired", err)
	}
}

func TestServiceRateLimitsAndLocksAfterFailedAttempts(t *testing.T) {
	service, err := NewService(NewMemoryStore(), sms.LocalSender{}, config.SMSConfig{
		CodeSecret:        "test-code-secret",
		CodeLength:        6,
		CodeTTL:           time.Minute,
		ResendInterval:    30 * time.Second,
		MaxVerifyAttempts: 2,
	}, "fallback-secret")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	result, err := service.Issue(ctx, "13800000000")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Issue(ctx, result.Phone); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("second Issue() error = %v, want rate limited", err)
	}
	if err := service.Verify(ctx, result.Phone, "000000"); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("first invalid Verify() error = %v", err)
	}
	if err := service.Verify(ctx, result.Phone, "000000"); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("second invalid Verify() error = %v", err)
	}
	if err := service.Verify(ctx, result.Phone, result.DebugCode); !errors.Is(err, ErrCodeExpired) {
		t.Fatalf("Verify() after lock error = %v, want expired", err)
	}
}
