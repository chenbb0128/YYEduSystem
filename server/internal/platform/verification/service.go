package verification

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/chenbb0128/tuoguan-system-server/internal/config"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/sms"
)

var (
	ErrInvalidPhone    = errors.New("invalid phone number")
	ErrCodeExpired     = errors.New("verification code expired")
	ErrInvalidCode     = errors.New("verification code is invalid")
	ErrTooManyAttempts = errors.New("too many verification attempts")
	ErrRateLimited     = errors.New("verification code resend too soon")
)

type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("verification code resend too soon (retry after %s)", e.RetryAfter)
}

func (e *RateLimitError) Unwrap() error { return ErrRateLimited }

type IssueResult struct {
	Phone      string
	ExpiresIn  int
	RetryAfter int
	DebugCode  string
}

type Service struct {
	store             Store
	sender            sms.Sender
	secret            []byte
	codeLength        int
	codeTTL           time.Duration
	resendInterval    time.Duration
	maxVerifyAttempts int
	clock             func() time.Time
}

func NewService(store Store, sender sms.Sender, cfg config.SMSConfig, fallbackSecret string) (*Service, error) {
	if store == nil {
		return nil, errors.New("verification store is required")
	}
	if sender == nil {
		return nil, errors.New("SMS sender is required")
	}
	secret := strings.TrimSpace(cfg.CodeSecret)
	if secret == "" {
		secret = strings.TrimSpace(fallbackSecret)
	}
	if secret == "" {
		return nil, errors.New("verification code secret is required")
	}
	codeLength := cfg.CodeLength
	if codeLength == 0 {
		codeLength = 6
	}
	codeTTL := cfg.CodeTTL
	if codeTTL <= 0 {
		codeTTL = 5 * time.Minute
	}
	resendInterval := cfg.ResendInterval
	if resendInterval <= 0 {
		resendInterval = time.Minute
	}
	maxAttempts := cfg.MaxVerifyAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	return &Service{
		store:             store,
		sender:            sender,
		secret:            []byte(secret),
		codeLength:        codeLength,
		codeTTL:           codeTTL,
		resendInterval:    resendInterval,
		maxVerifyAttempts: maxAttempts,
		clock:             time.Now,
	}, nil
}

func (s *Service) Local() bool { return s.sender.Local() }

func (s *Service) Issue(ctx context.Context, phone string) (IssueResult, error) {
	phone = normalizePhone(phone)
	if phone == "" {
		return IssueResult{}, ErrInvalidPhone
	}
	phoneHash := s.phoneHash(phone)
	cooldownKey := "auth:phone-code:cooldown:" + phoneHash
	acquired, retryAfter, err := s.store.Acquire(ctx, cooldownKey, []byte("1"), s.resendInterval)
	if err != nil {
		return IssueResult{}, fmt.Errorf("acquire verification cooldown: %w", err)
	}
	if !acquired {
		return IssueResult{}, &RateLimitError{RetryAfter: retryAfter}
	}

	code, err := randomDigits(s.codeLength)
	if err != nil {
		_ = s.store.Delete(ctx, cooldownKey)
		return IssueResult{}, fmt.Errorf("generate verification code: %w", err)
	}
	now := s.clock()
	record, err := json.Marshal(codeRecord{Hash: s.codeHash(phone, code), ExpiresAt: now.Add(s.codeTTL)})
	if err != nil {
		_ = s.store.Delete(ctx, cooldownKey)
		return IssueResult{}, fmt.Errorf("encode verification code: %w", err)
	}
	codeKey := "auth:phone-code:" + phoneHash
	if err := s.store.Set(ctx, codeKey, record, s.codeTTL); err != nil {
		_ = s.store.Delete(ctx, cooldownKey)
		return IssueResult{}, fmt.Errorf("store verification code: %w", err)
	}
	if err := s.sender.Send(ctx, phone, code); err != nil {
		_ = s.store.Delete(ctx, codeKey)
		_ = s.store.Delete(ctx, cooldownKey)
		return IssueResult{}, err
	}
	result := IssueResult{Phone: phone, ExpiresIn: secondsCeil(s.codeTTL), RetryAfter: secondsCeil(s.resendInterval)}
	if s.Local() {
		result.DebugCode = code
	}
	return result, nil
}

func (s *Service) Verify(ctx context.Context, phone, code string) error {
	phone = normalizePhone(phone)
	if phone == "" {
		return ErrInvalidPhone
	}
	codeKey := "auth:phone-code:" + s.phoneHash(phone)
	value, err := s.store.Get(ctx, codeKey)
	if errors.Is(err, ErrNotFound) {
		return ErrCodeExpired
	}
	if err != nil {
		return fmt.Errorf("read verification code: %w", err)
	}
	var record codeRecord
	if err := json.Unmarshal(value, &record); err != nil {
		_ = s.store.Delete(ctx, codeKey)
		return ErrCodeExpired
	}
	if !record.ExpiresAt.After(s.clock()) {
		_ = s.store.Delete(ctx, codeKey)
		return ErrCodeExpired
	}
	expected := s.codeHash(phone, strings.TrimSpace(code))
	if subtle.ConstantTimeCompare([]byte(record.Hash), []byte(expected)) != 1 {
		record.Attempts++
		if record.Attempts >= s.maxVerifyAttempts {
			_ = s.store.Delete(ctx, codeKey)
			return ErrTooManyAttempts
		}
		remaining := record.ExpiresAt.Sub(s.clock())
		if remaining > 0 {
			if update, marshalErr := json.Marshal(record); marshalErr == nil {
				_ = s.store.Set(ctx, codeKey, update, remaining)
			}
		}
		return ErrInvalidCode
	}
	if err := s.store.Delete(ctx, codeKey); err != nil {
		return fmt.Errorf("consume verification code: %w", err)
	}
	return nil
}

type codeRecord struct {
	Hash      string    `json:"hash"`
	Attempts  int       `json:"attempts"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Service) phoneHash(phone string) string {
	digest := hmac.New(sha256.New, s.secret)
	_, _ = digest.Write([]byte(phone))
	return hex.EncodeToString(digest.Sum(nil))
}

func (s *Service) codeHash(phone, code string) string {
	digest := hmac.New(sha256.New, s.secret)
	_, _ = digest.Write([]byte(phone))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(code))
	return hex.EncodeToString(digest.Sum(nil))
}

func normalizePhone(value string) string {
	var builder strings.Builder
	for _, char := range strings.TrimSpace(value) {
		if char >= '0' && char <= '9' {
			builder.WriteRune(char)
		}
	}
	phone := builder.String()
	if len(phone) < 7 || len(phone) > 32 {
		return ""
	}
	return phone
}

func randomDigits(length int) (string, error) {
	var builder strings.Builder
	builder.Grow(length)
	max := big.NewInt(10)
	for index := 0; index < length; index++ {
		digit, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(byte('0' + digit.Int64()))
	}
	return builder.String(), nil
}

func secondsCeil(duration time.Duration) int {
	if duration <= 0 {
		return 0
	}
	seconds := int(duration / time.Second)
	if duration%time.Second != 0 {
		seconds++
	}
	return seconds
}
