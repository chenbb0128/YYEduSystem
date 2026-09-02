package storage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestS3StoreSavesAndReadsPrivateObject(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/test-bucket/")
		if r.Method == http.MethodPut {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read PUT body: %v", err)
			}
			mu.Lock()
			objects[key] = body
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return
		}
		mu.Lock()
		body, ok := objects[key]
		mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(body)
	}))
	defer server.Close()

	store, err := NewS3Store(S3Config{Endpoint: server.URL, Bucket: "test-bucket", Region: "test-1", AccessKey: "access", SecretKey: "secret", PathStyle: true, MaxBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte{0xff, 0xd8, 0xff, 0xe0, 'f', 'a', 'k', 'e'}
	asset, err := store.SaveIn(context.Background(), "meals", "today.jpg", "image/jpeg", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if asset.Key == "" || !strings.HasPrefix(asset.URL, "/uploads/meals/") || asset.SHA256 == "" {
		t.Fatalf("asset metadata = %+v", asset)
	}
	reader, contentType, err := store.OpenURL(asset.URL + "?expires=1&sig=test")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, payload) || contentType != "image/jpeg" {
		t.Fatalf("read object = %q, %q", data, contentType)
	}
}

func TestS3StoreRejectsOutsidePublicPath(t *testing.T) {
	store, err := NewS3Store(S3Config{Endpoint: "http://127.0.0.1:1", Bucket: "test", Region: "test-1", AccessKey: "access", SecretKey: "secret", PathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.OpenURL("/other/file.jpg"); err == nil {
		t.Fatal("OpenURL accepted an object outside the public path")
	}
}
