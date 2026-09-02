package storage

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestLocalFileStoreSavesImageAndRejectsOversizedFile(t *testing.T) {
	store, err := NewLocalFileStore(t.TempDir(), "/uploads", 16)
	if err != nil {
		t.Fatal(err)
	}
	jpegHeader := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01}
	asset, err := store.Save(context.Background(), "gate.jpg", "image/jpeg", bytes.NewReader(jpegHeader))
	if err != nil {
		t.Fatal(err)
	}
	if asset.URL == "" || asset.ContentType != "image/jpeg" || asset.Size != int64(len(jpegHeader)) {
		t.Fatalf("asset = %+v", asset)
	}
	if _, err := store.Save(context.Background(), "gate.jpg", "image/jpeg", bytes.NewReader(bytes.Repeat([]byte{'x'}, 17))); err != ErrFileTooLarge {
		t.Fatalf("oversized error = %v, want %v", err, ErrFileTooLarge)
	}
	if _, err := store.Save(context.Background(), "gate.txt", "text/plain", strings.NewReader("1234")); err != ErrUnsupportedContentType {
		t.Fatalf("content type error = %v, want %v", err, ErrUnsupportedContentType)
	}
}

func TestSignedAssetURLAndOpenURL(t *testing.T) {
	store, err := NewLocalFileStore(t.TempDir(), "/uploads", DefaultMaxPhotoBytes)
	if err != nil {
		t.Fatal(err)
	}
	asset, err := store.SaveIn(context.Background(), "homework", "task.png", "image/png", bytes.NewReader([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}))
	if err != nil {
		t.Fatal(err)
	}
	reader, contentType, err := store.OpenURL(asset.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.(io.Closer).Close()
	if contentType != "image/png" {
		t.Fatalf("content type = %q, want image/png", contentType)
	}
	signer, err := NewURLSigner("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	signed := signer.Sign(asset.URL, time.Minute)
	parsed, err := url.Parse(signed)
	if err != nil {
		t.Fatal(err)
	}
	if !signer.Verify(parsed.Path, parsed.Query().Get("expires"), parsed.Query().Get("sig"), time.Now()) {
		t.Fatal("signed URL did not verify")
	}
	if signer.Verify(parsed.Path, parsed.Query().Get("expires"), "invalid", time.Now()) {
		t.Fatal("invalid signature verified")
	}
}
