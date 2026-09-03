package sms

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chenbb0128/tuoguan-system-server/internal/config"
)

const (
	tencentService = "sms"
	tencentVersion = "2021-01-11"
)

type TencentSender struct {
	secretID  string
	secretKey string
	sdkAppID  string
	signName  string
	template  string
	region    string
	endpoint  *url.URL
	client    *http.Client
	now       func() time.Time
}

func (s *TencentSender) Local() bool { return false }

func NewTencentSender(cfg config.SMSConfig) (*TencentSender, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = "https://sms.tencentcloudapi.com/"
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid Tencent SMS endpoint %q", endpoint)
	}
	for name, value := range map[string]string{
		"secret_id":   cfg.SecretID,
		"secret_key":  cfg.SecretKey,
		"sdk_app_id":  cfg.SDKAppID,
		"sign_name":   cfg.SignName,
		"template_id": cfg.TemplateID,
		"region":      cfg.Region,
	} {
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("Tencent SMS %s is required", name)
		}
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &TencentSender{
		secretID:  strings.TrimSpace(cfg.SecretID),
		secretKey: cfg.SecretKey,
		sdkAppID:  strings.TrimSpace(cfg.SDKAppID),
		signName:  strings.TrimSpace(cfg.SignName),
		template:  strings.TrimSpace(cfg.TemplateID),
		region:    strings.TrimSpace(cfg.Region),
		endpoint:  parsed,
		client:    &http.Client{Timeout: timeout},
		now:       time.Now,
	}, nil
}

type sendSMSRequest struct {
	SmsSdkAppID      string   `json:"SmsSdkAppId"`
	SignName         string   `json:"SignName"`
	TemplateID       string   `json:"TemplateId"`
	TemplateParamSet []string `json:"TemplateParamSet"`
	PhoneNumberSet   []string `json:"PhoneNumberSet"`
}

type tencentError struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

type sendSMSResponse struct {
	Response struct {
		Error     *tencentError `json:"Error"`
		RequestID string        `json:"RequestId"`
	} `json:"Response"`
}

func (s *TencentSender) Send(ctx context.Context, phone, code string) error {
	payload, err := json.Marshal(sendSMSRequest{
		SmsSdkAppID:      s.sdkAppID,
		SignName:         s.signName,
		TemplateID:       s.template,
		TemplateParamSet: []string{code},
		PhoneNumberSet:   []string{"+86" + phone},
	})
	if err != nil {
		return fmt.Errorf("marshal Tencent SMS request: %w", err)
	}

	now := s.now().UTC()
	timestamp := now.Unix()
	date := now.Format("2006-01-02")
	host := s.endpoint.Host
	contentType := "application/json; charset=utf-8"
	hashedPayload := sha256Hex(payload)
	canonicalRequest := strings.Join([]string{
		"POST",
		s.defaultPath(),
		"",
		"content-type:" + contentType,
		"host:" + host,
		"",
		"content-type;host",
		hashedPayload,
	}, "\n")
	hashedCanonicalRequest := sha256Hex([]byte(canonicalRequest))
	credentialScope := date + "/" + tencentService + "/tc3_request"
	stringToSign := strings.Join([]string{
		"TC3-HMAC-SHA256",
		fmt.Sprintf("%d", timestamp),
		credentialScope,
		hashedCanonicalRequest,
	}, "\n")
	secretDate := hmacSHA256([]byte("TC3"+s.secretKey), date)
	secretService := hmacSHA256(secretDate, tencentService)
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))
	authorization := fmt.Sprintf(
		"TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=content-type;host, Signature=%s",
		s.secretID, credentialScope, signature,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create Tencent SMS request: %w", err)
	}
	req.Host = host
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-TC-Action", "SendSms")
	req.Header.Set("X-TC-Version", tencentVersion)
	req.Header.Set("X-TC-Region", s.region)
	req.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-TC-Language", "zh-CN")
	req.Header.Set("Authorization", authorization)

	response, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send Tencent SMS request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read Tencent SMS response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Tencent SMS returned HTTP %d", response.StatusCode)
	}
	var result sendSMSResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode Tencent SMS response: %w", err)
	}
	if result.Response.Error != nil {
		return fmt.Errorf("Tencent SMS rejected request: %s: %s", result.Response.Error.Code, result.Response.Error.Message)
	}
	return nil
}

func (s *TencentSender) defaultPath() string {
	path := s.endpoint.EscapedPath()
	if path == "" {
		return "/"
	}
	return path
}

func sha256Hex(value []byte) string {
	hash := sha256.Sum256(value)
	return hex.EncodeToString(hash[:])
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}
