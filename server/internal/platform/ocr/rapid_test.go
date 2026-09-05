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

func TestRapidClientExtractTextPostsImageAndDecodesLines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/ocr" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		var payload rapidOCRRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.ContentType != "image/png" {
			t.Fatalf("content type = %q", payload.ContentType)
		}
		data, err := base64.StdEncoding.DecodeString(payload.ImageBase64)
		if err != nil {
			t.Fatalf("ImageBase64 is invalid: %v", err)
		}
		if string(data) != "fake-image" {
			t.Fatalf("uploaded image = %q", string(data))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":0,"lines":[{"text":"1. 12+8=20","confidence":0.91},{"text":"2. 48÷6=8","confidence":0.89}]}`))
	}))
	defer server.Close()

	client, err := NewRapidClient(config.OCRConfig{
		Endpoint:      server.URL + "/ocr",
		Timeout:       time.Second,
		MaxImageBytes: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	client.httpClient = server.Client()

	result, err := client.ExtractText(context.Background(), strings.NewReader("fake-image"), "image/png")
	if err != nil {
		t.Fatalf("ExtractText() error = %v", err)
	}
	if result.Provider != "rapidocr" || len(result.Lines) != 2 || !strings.Contains(result.Text, "48÷6") {
		t.Fatalf("result = %+v", result)
	}
	if result.Confidence < 0.89 || result.Confidence > 0.91 {
		t.Fatalf("confidence = %f", result.Confidence)
	}
}

func TestRapidClientUsesPlainTextWhenLinesAreMissing(t *testing.T) {
	result := rapidToTextResult(rapidOCRResponse{Text: "1. 口算\n2. 填空"})
	if len(result.Lines) != 2 || !strings.Contains(result.Text, "填空") {
		t.Fatalf("result = %+v", result)
	}
}
