package sms

import (
	"context"
	"fmt"
	"strings"

	"github.com/chenbb0128/tuoguan-system-server/internal/config"
)

// Sender delivers a one-time verification code. LocalSender deliberately does
// not send anything; it is only used by local development, where the API may
// return the generated code in a debug-only response field.
type Sender interface {
	Send(context.Context, string, string) error
	Local() bool
}

type LocalSender struct{}

func (LocalSender) Send(context.Context, string, string) error { return nil }

func (LocalSender) Local() bool { return true }

func NewSender(cfg config.SMSConfig) (Sender, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "local"
	}
	if !cfg.Enabled || provider == "local" {
		return LocalSender{}, nil
	}
	if provider != "tencent" {
		return nil, fmt.Errorf("unsupported SMS provider %q", provider)
	}
	return NewTencentSender(cfg)
}
