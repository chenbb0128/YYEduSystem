package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type URLSigner struct {
	secret []byte
}

func NewURLSigner(secret string) (*URLSigner, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, fmt.Errorf("storage url signing secret is required")
	}
	return &URLSigner{secret: []byte(secret)}, nil
}

func (s *URLSigner) Sign(rawURL string, ttl time.Duration) string {
	if s == nil || strings.TrimSpace(rawURL) == "" {
		return rawURL
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Path == "" {
		return rawURL
	}
	expires := time.Now().Add(ttl).Unix()
	query := parsed.Query()
	query.Set("expires", strconv.FormatInt(expires, 10))
	query.Set("sig", s.signature(parsed.Path, expires))
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func (s *URLSigner) Verify(path, expires, signature string, now time.Time) bool {
	if s == nil || strings.TrimSpace(path) == "" {
		return false
	}
	expiresAt, err := strconv.ParseInt(strings.TrimSpace(expires), 10, 64)
	if err != nil || expiresAt < now.Unix() {
		return false
	}
	expected := s.signature(path, expiresAt)
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signature)))
}

func (s *URLSigner) signature(path string, expires int64) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(path + "\n" + strconv.FormatInt(expires, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}
