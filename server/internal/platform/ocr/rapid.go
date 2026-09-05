package ocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chenbb0128/tuoguan-system-server/internal/config"
)

type RapidClient struct {
	endpoint      *url.URL
	maxImageBytes int64
	httpClient    *http.Client
}

func NewRapidClient(cfg config.OCRConfig) (*RapidClient, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = "http://127.0.0.1:9009/ocr"
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("invalid RapidOCR endpoint %q", endpoint)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	maxImageBytes := cfg.MaxImageBytes
	if maxImageBytes <= 0 {
		maxImageBytes = 5 << 20
	}
	return &RapidClient{
		endpoint:      parsed,
		maxImageBytes: maxImageBytes,
		httpClient:    &http.Client{Timeout: timeout},
	}, nil
}

type rapidOCRRequest struct {
	ImageBase64 string `json:"image_base64"`
	ContentType string `json:"content_type,omitempty"`
}

type rapidOCRLine struct {
	Text         string  `json:"text"`
	DetectedText string  `json:"detected_text"`
	Confidence   float64 `json:"confidence"`
	Score        float64 `json:"score"`
}

type rapidOCRResponse struct {
	Code    *int           `json:"code"`
	Message string         `json:"message"`
	Error   string         `json:"error"`
	Text    string         `json:"text"`
	Lines   []rapidOCRLine `json:"lines"`
	Items   []rapidOCRLine `json:"items"`
	Results []rapidOCRLine `json:"results"`
}

func (c *RapidClient) ExtractText(ctx context.Context, image io.Reader, contentType string) (TextResult, error) {
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
	payload, err := json.Marshal(rapidOCRRequest{
		ImageBase64: base64.StdEncoding.EncodeToString(data),
		ContentType: strings.TrimSpace(contentType),
	})
	if err != nil {
		return TextResult{}, fmt.Errorf("marshal RapidOCR request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return TextResult{}, fmt.Errorf("create RapidOCR request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return TextResult{}, fmt.Errorf("send RapidOCR request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return TextResult{}, fmt.Errorf("read RapidOCR response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return TextResult{}, fmt.Errorf("RapidOCR returned HTTP %d", response.StatusCode)
	}
	var result rapidOCRResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return TextResult{}, fmt.Errorf("decode RapidOCR response: %w", err)
	}
	if result.Code != nil && *result.Code != 0 {
		return TextResult{}, fmt.Errorf("RapidOCR rejected request: %s", firstNonEmpty(result.Message, result.Error, "unknown error"))
	}
	if strings.TrimSpace(result.Error) != "" {
		return TextResult{}, fmt.Errorf("RapidOCR rejected request: %s", result.Error)
	}
	return rapidToTextResult(result), nil
}

func rapidToTextResult(result rapidOCRResponse) TextResult {
	rawLines := result.Lines
	if len(rawLines) == 0 {
		rawLines = result.Items
	}
	if len(rawLines) == 0 {
		rawLines = result.Results
	}
	lines := make([]TextLine, 0, len(rawLines))
	var text strings.Builder
	var confidenceTotal float64
	var confidenceCount int
	for _, raw := range rawLines {
		value := firstNonEmpty(raw.Text, raw.DetectedText)
		if value == "" {
			continue
		}
		confidence := normalizeConfidence(firstPositive(raw.Confidence, raw.Score))
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
	if text.Len() == 0 {
		for _, line := range strings.Split(strings.TrimSpace(result.Text), "\n") {
			value := strings.TrimSpace(line)
			if value == "" {
				continue
			}
			lines = append(lines, TextLine{Text: value})
			if text.Len() > 0 {
				text.WriteString("\n")
			}
			text.WriteString(value)
		}
	}
	averageConfidence := 0.0
	if confidenceCount > 0 {
		averageConfidence = confidenceTotal / float64(confidenceCount)
	}
	return TextResult{
		Provider:   "rapidocr",
		Action:     "rapidocr",
		Text:       text.String(),
		Lines:      lines,
		Confidence: averageConfidence,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstPositive(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
