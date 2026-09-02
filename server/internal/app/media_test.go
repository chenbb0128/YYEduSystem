package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/chenbb0128/tuoguan-system-server/internal/platform/storage"
)

func TestProtectedUploadHandlerRequiresValidSignature(t *testing.T) {
	store, err := storage.NewLocalFileStore(t.TempDir(), "/uploads", storage.DefaultMaxPhotoBytes)
	if err != nil {
		t.Fatal(err)
	}
	asset, err := store.SaveIn(context.Background(), "pickup", "arrival.jpg", "image/jpeg", bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xe0}))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := storage.NewURLSigner("media-test-secret")
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/uploads/*path", protectedUploadHandler(signer, store))
	router.HEAD("/uploads/*path", protectedUploadHandler(signer, store))

	validURL := signer.Sign(asset.URL, time.Minute)
	t.Run("valid", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, validURL, nil)
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), []byte{0xff, 0xd8, 0xff, 0xe0}) {
			t.Fatalf("status = %d, body = %v", rec.Code, rec.Body.Bytes())
		}
		if got := rec.Header().Get("Content-Type"); got != "image/jpeg" {
			t.Fatalf("content type = %q", got)
		}
	})

	t.Run("unsigned", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, asset.URL, nil)
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
	})

	t.Run("expired", func(t *testing.T) {
		parsed, parseErr := url.Parse(validURL)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		query := parsed.Query()
		query.Set("expires", "1")
		parsed.RawQuery = query.Encode()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, parsed.String(), nil)
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
	})

	t.Run("missing-file", func(t *testing.T) {
		missingURL := signer.Sign("/uploads/pickup/missing.jpg", time.Minute)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, missingURL, nil)
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("path-tampering", func(t *testing.T) {
		parsed, parseErr := url.Parse(validURL)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		parsed.Path = "/uploads/pickup/other.jpg"
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, parsed.String(), nil)
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
	})

	t.Run("head", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodHead, validURL, nil)
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
			t.Fatalf("status = %d, body length = %d", rec.Code, rec.Body.Len())
		}
	})
}
