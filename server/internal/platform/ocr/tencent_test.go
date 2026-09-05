package ocr

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chenbb0128/tuoguan-system-server/internal/config"
)

func TestTencentClientExtractTextSignsAndDecodesRequest(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("X-TC-Action") != "GeneralHandwritingOCR" || request.Header.Get("X-TC-Version") != tencentVersion {
			t.Fatalf("Tencent headers = %#v", request.Header)
		}
		if !strings.HasPrefix(request.Header.Get("Authorization"), "TC3-HMAC-SHA256 Credential=test-id/") {
			t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
		}
		var payload generalHandwritingOCRRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		data, err := base64.StdEncoding.DecodeString(payload.ImageBase64)
		if err != nil {
			t.Fatalf("ImageBase64 is invalid: %v", err)
		}
		if string(data) != "fake-image" {
			t.Fatalf("uploaded image = %q", string(data))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"Response":{"TextDetections":[{"DetectedText":"1. 12+8=20","Confidence":98.5},{"DetectedText":"2. 48÷6=8","Confidence":97}],"RequestId":"request-id"}}`))
	}))
	defer server.Close()

	client, err := NewTencentClient(config.OCRConfig{
		SecretID:      "test-id",
		SecretKey:     "test-key",
		Region:        "ap-guangzhou",
		Endpoint:      server.URL,
		Timeout:       time.Second,
		Action:        "GeneralHandwritingOCR",
		MaxImageBytes: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	client.httpClient = server.Client()
	client.now = func() time.Time { return time.Date(2026, 9, 4, 8, 9, 10, 0, time.UTC) }

	result, err := client.ExtractText(context.Background(), strings.NewReader("fake-image"), "image/jpeg")
	if err != nil {
		t.Fatalf("ExtractText() error = %v", err)
	}
	if result.Provider != "tencent" || result.Action != "GeneralHandwritingOCR" {
		t.Fatalf("provider/action = %s/%s", result.Provider, result.Action)
	}
	if len(result.Lines) != 2 || !strings.Contains(result.Text, "12+8") || result.Confidence < 0.97 {
		t.Fatalf("result = %+v", result)
	}
}

func TestTencentClientRejectsLargeImage(t *testing.T) {
	client, err := NewTencentClient(config.OCRConfig{
		SecretID:      "test-id",
		SecretKey:     "test-key",
		Region:        "ap-guangzhou",
		Endpoint:      "https://ocr.tencentcloudapi.com/",
		Timeout:       time.Second,
		Action:        "GeneralHandwritingOCR",
		MaxImageBytes: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ExtractText(context.Background(), strings.NewReader("12345"), "image/jpeg")
	if err == nil || !strings.Contains(err.Error(), ErrImageTooLarge.Error()) {
		t.Fatalf("ExtractText() error = %v, want image too large", err)
	}
}
