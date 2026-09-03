package sms

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chenbb0128/tuoguan-system-server/internal/config"
)

func TestTencentSenderSignsAndBuildsRequest(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("X-TC-Action") != "SendSms" || request.Header.Get("X-TC-Version") != tencentVersion {
			t.Fatalf("Tencent headers = %#v", request.Header)
		}
		if !strings.HasPrefix(request.Header.Get("Authorization"), "TC3-HMAC-SHA256 Credential=test-id/") {
			t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
		}
		var payload sendSMSRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.SmsSdkAppID != "app-id" || payload.SignName != "豆芽成长" || payload.TemplateID != "100001" || payload.PhoneNumberSet[0] != "+8613800000000" || payload.TemplateParamSet[0] != "654321" {
			t.Fatalf("payload = %+v", payload)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"Response":{"RequestId":"request-id"}}`))
	}))
	defer server.Close()

	sender, err := NewTencentSender(config.SMSConfig{
		SecretID:   "test-id",
		SecretKey:  "test-key",
		SDKAppID:   "app-id",
		SignName:   "豆芽成长",
		TemplateID: "100001",
		Region:     "ap-guangzhou",
		Endpoint:   server.URL,
		Timeout:    time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	sender.client = server.Client()
	sender.now = func() time.Time { return time.Date(2026, 9, 3, 1, 2, 3, 0, time.UTC) }
	if err := sender.Send(context.Background(), "13800000000", "654321"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
}

func TestTencentSignatureHelperMatchesKnownDigest(t *testing.T) {
	got := sha256Hex([]byte("hello"))
	wantBytes := sha256.Sum256([]byte("hello"))
	if got != hex.EncodeToString(wantBytes[:]) {
		t.Fatalf("sha256Hex() = %q", got)
	}
	mac := hmac.New(sha256.New, []byte("key"))
	_, _ = mac.Write([]byte("value"))
	if !hmac.Equal(hmacSHA256([]byte("key"), "value"), mac.Sum(nil)) {
		t.Fatal("hmacSHA256() does not match standard HMAC")
	}
}
