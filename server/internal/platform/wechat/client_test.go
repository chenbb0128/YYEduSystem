package wechat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientExchangeCode(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("appid") != "test-app" || query.Get("secret") != "test-secret" || query.Get("js_code") != "one-time-code" || query.Get("grant_type") != "authorization_code" {
			t.Fatalf("code2session query = %v", query)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"openid":"openid-test","session_key":"session-test"}`))
	}))
	defer server.Close()

	client := newTestClient(t, server)
	openID, err := client.ExchangeCode(context.Background(), "one-time-code")
	if err != nil {
		t.Fatal(err)
	}
	if openID != "openid-test" {
		t.Fatalf("openID = %q, want openid-test", openID)
	}
}

func TestClientSendSubscribeMessage(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/cgi-bin/token":
			if r.URL.Query().Get("grant_type") != "client_credential" {
				t.Fatalf("token query = %v", r.URL.Query())
			}
			_, _ = w.Write([]byte(`{"access_token":"access-token","expires_in":7200}`))
		case "/cgi-bin/message/subscribe/send":
			if r.URL.Query().Get("access_token") != "access-token" {
				t.Fatalf("subscribe query = %v", r.URL.Query())
			}
			var body subscribeMessageRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode subscribe request: %v", err)
			}
			if body.ToUser != "openid-test" || body.TemplateID != "template-test" || body.Page != "pages/parent/index" || body.MiniprogramState != "formal" {
				t.Fatalf("subscribe body = %+v", body)
			}
			if body.Data["thing1"].Value != "接送提醒" || body.Data["time3"].Value == "" {
				t.Fatalf("subscribe data = %+v", body.Data)
			}
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server)
	err := client.SendSubscribeMessage(context.Background(), SubscribeMessageParams{
		ToUser:     "openid-test",
		TemplateID: "template-test",
		Page:       "pages/parent/index",
		Data:       map[string]string{"thing1": "接送提醒", "time3": time.Now().Format(time.RFC3339)},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientGenerateMiniProgramCode(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/cgi-bin/token":
			_, _ = w.Write([]byte(`{"access_token":"access-token","expires_in":7200}`))
		case "/wxa/getwxacodeunlimit":
			if r.URL.Query().Get("access_token") != "access-token" {
				t.Fatalf("mini-program code token = %q", r.URL.Query().Get("access_token"))
			}
			var body struct {
				Scene      string `json:"scene"`
				Page       string `json:"page"`
				EnvVersion string `json:"env_version"`
				Width      int    `json:"width"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode mini-program code request: %v", err)
			}
			if body.Scene != "cabc.12345678" || body.Page != "pages/parent/index" || body.EnvVersion != "trial" || body.Width != 430 {
				t.Fatalf("mini-program code body = %+v", body)
			}
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png-bytes"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server)
	image, err := client.GenerateMiniProgramCode(context.Background(), MiniProgramCodeParams{Scene: "cabc.12345678", Page: "pages/parent/index", EnvVersion: "trial"})
	if err != nil {
		t.Fatal(err)
	}
	if string(image) != "png-bytes" {
		t.Fatalf("image = %q", image)
	}
}

func TestNewClientRequiresHTTPS(t *testing.T) {
	if _, err := NewClient("app", "secret", "http://api.weixin.qq.com/sns/jscode2session", time.Second); err == nil {
		t.Fatal("NewClient accepted an HTTP endpoint")
	}
}

func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	client, err := NewClient("test-app", "test-secret", "https://api.weixin.qq.com/sns/jscode2session", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	client.endpoint = strings.TrimRight(server.URL, "/") + "/sns/jscode2session"
	client.baseURL = server.URL
	client.http = server.Client()
	return client
}
