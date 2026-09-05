package ocr

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chenbb0128/tuoguan-system-server/internal/config"
)

const (
	tencentService = "ocr"
	tencentVersion = "2018-11-19"
)

var (
	ErrImageTooLarge = errors.New("ocr: image file is too large")
	ErrEmptyImage    = errors.New("ocr: image file is empty")
)

type TencentClient struct {
	secretID      string
	secretKey     string
	region        string
	endpoint      *url.URL
	action        string
	maxImageBytes int64
	httpClient    *http.Client
	now           func() time.Time
}

func NewTencentClient(cfg config.OCRConfig) (*TencentClient, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = "https://ocr.tencentcloudapi.com/"
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid Tencent OCR endpoint %q", endpoint)
	}
	for name, value := range map[string]string{
		"secret_id":  cfg.SecretID,
		"secret_key": cfg.SecretKey,
		"region":     cfg.Region,
	} {
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("Tencent OCR %s is required", name)
		}
	}
	action := strings.TrimSpace(cfg.Action)
	if action == "" {
		action = "GeneralHandwritingOCR"
	}
	if action != "GeneralHandwritingOCR" {
		return nil, fmt.Errorf("Tencent OCR action %q is not supported", action)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	maxImageBytes := cfg.MaxImageBytes
	if maxImageBytes <= 0 {
		maxImageBytes = 5 << 20
	}
	return &TencentClient{
		secretID:      strings.TrimSpace(cfg.SecretID),
		secretKey:     cfg.SecretKey,
		region:        strings.TrimSpace(cfg.Region),
		endpoint:      parsed,
		action:        action,
		maxImageBytes: maxImageBytes,
		httpClient:    &http.Client{Timeout: timeout},
		now:           time.Now,
	}, nil
}

type generalHandwritingOCRRequest struct {
	ImageBase64       string `json:"ImageBase64"`
	EnableWordPolygon bool   `json:"EnableWordPolygon,omitempty"`
}

type tencentTextDetection struct {
	DetectedText string  `json:"DetectedText"`
	Confidence   float64 `json:"Confidence"`
}

type tencentError struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

type generalHandwritingOCRResponse struct {
	Response struct {
		TextDetections []tencentTextDetection `json:"TextDetections"`
		Error          *tencentError          `json:"Error"`
		RequestID      string                 `json:"RequestId"`
	} `json:"Response"`
}

func (c *TencentClient) ExtractText(ctx context.Context, image io.Reader, contentType string) (TextResult, error) {
	_ = contentType
	if image == nil {
		return TextResult{}, ErrEmptyImage
	}
	data, err := io.ReadAll(io.LimitReader(image, c.maxImageBytes+1))
	if err != nil {
		return TextResult{}, fmt.Errorf("read OCR image: %w", err)
	}
	if int64(len(data)) > c.maxImageBytes {
		return TextResult{}, ErrImageTooLarge
	}
	if len(data) == 0 {
		return TextResult{}, ErrEmptyImage
	}
	payload, err := json.Marshal(generalHandwritingOCRRequest{
		ImageBase64: base64.StdEncoding.EncodeToString(data),
	})
	if err != nil {
		return TextResult{}, fmt.Errorf("marshal Tencent OCR request: %w", err)
	}
	request, err := c.signedRequest(ctx, payload)
	if err != nil {
		return TextResult{}, err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return TextResult{}, fmt.Errorf("send Tencent OCR request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return TextResult{}, fmt.Errorf("read Tencent OCR response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return TextResult{}, fmt.Errorf("Tencent OCR returned HTTP %d", response.StatusCode)
	}
	var result generalHandwritingOCRResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return TextResult{}, fmt.Errorf("decode Tencent OCR response: %w", err)
	}
	if result.Response.Error != nil {
		return TextResult{}, fmt.Errorf("Tencent OCR rejected request: %s: %s", result.Response.Error.Code, result.Response.Error.Message)
	}
	return toTextResult(result.Response.TextDetections, "tencent", c.action), nil
}

func (c *TencentClient) signedRequest(ctx context.Context, payload []byte) (*http.Request, error) {
	now := c.now().UTC()
	timestamp := now.Unix()
	date := now.Format("2006-01-02")
	host := c.endpoint.Host
	contentType := "application/json; charset=utf-8"
	hashedPayload := sha256Hex(payload)
	canonicalRequest := strings.Join([]string{
		http.MethodPost,
		c.defaultPath(),
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
	secretDate := hmacSHA256([]byte("TC3"+c.secretKey), date)
	secretService := hmacSHA256(secretDate, tencentService)
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))
	authorization := fmt.Sprintf(
		"TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=content-type;host, Signature=%s",
		c.secretID, credentialScope, signature,
	)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create Tencent OCR request: %w", err)
	}
	request.Host = host
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("X-TC-Action", c.action)
	request.Header.Set("X-TC-Version", tencentVersion)
	request.Header.Set("X-TC-Region", c.region)
	request.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))
	request.Header.Set("X-TC-Language", "zh-CN")
	request.Header.Set("Authorization", authorization)
	return request, nil
}

func (c *TencentClient) defaultPath() string {
	path := c.endpoint.EscapedPath()
	if path == "" {
		return "/"
	}
	return path
}

func toTextResult(detections []tencentTextDetection, provider, action string) TextResult {
	lines := make([]TextLine, 0, len(detections))
	var text strings.Builder
	var confidenceTotal float64
	var confidenceCount int
	for _, detection := range detections {
		value := strings.TrimSpace(detection.DetectedText)
		if value == "" {
			continue
		}
		confidence := normalizeConfidence(detection.Confidence)
		lines = append(lines, TextLine{Text: value, Confidence: confidence})
		if text.Len() > 0 {
			text.WriteString("\n")
		}
		text.WriteString(value)
		if confidence > 0 {
			confidenceTotal += confidence
			confidenceCount++
		}
	}
	averageConfidence := 0.0
	if confidenceCount > 0 {
		averageConfidence = confidenceTotal / float64(confidenceCount)
	}
	return TextResult{
		Provider:   provider,
		Action:     action,
		Text:       text.String(),
		Lines:      lines,
		Confidence: averageConfidence,
	}
}

func normalizeConfidence(value float64) float64 {
	if value > 1 {
		value = value / 100
	}
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
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
