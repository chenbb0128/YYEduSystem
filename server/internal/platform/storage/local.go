package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const DefaultMaxPhotoBytes int64 = 5 * 1024 * 1024

var (
	ErrUnsupportedContentType = errors.New("storage: unsupported content type")
	ErrFileTooLarge           = errors.New("storage: file too large")
)

type Asset struct {
	URL         string
	Key         string
	SHA256      string
	ContentType string
	Size        int64
}

type Store interface {
	Save(context.Context, string, string, io.Reader) (Asset, error)
}

type FileReader interface {
	OpenURL(string) (io.ReadSeeker, string, error)
}

type LocalFileStore struct {
	rootDir    string
	publicPath string
	maxBytes   int64
}

func NewLocalFileStore(rootDir, publicPath string, maxBytes int64) (*LocalFileStore, error) {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		return nil, fmt.Errorf("storage upload directory is required")
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxPhotoBytes
	}
	if err := os.MkdirAll(filepath.Join(rootDir, "pickup"), 0o755); err != nil {
		return nil, fmt.Errorf("create upload directory: %w", err)
	}
	publicPath = strings.TrimRight(strings.TrimSpace(publicPath), "/")
	if publicPath == "" {
		publicPath = "/uploads"
	}
	return &LocalFileStore{rootDir: rootDir, publicPath: publicPath, maxBytes: maxBytes}, nil
}

func (s *LocalFileStore) Save(ctx context.Context, filename, contentType string, reader io.Reader) (Asset, error) {
	return s.SaveIn(ctx, "pickup", filename, contentType, reader)
}

// SaveIn stores an image in a named first-level category.
func (s *LocalFileStore) SaveIn(ctx context.Context, category, filename, contentType string, reader io.Reader) (Asset, error) {
	category = strings.TrimSpace(category)
	if category == "" || category == "." || category == ".." || filepath.Base(category) != category {
		return Asset{}, fmt.Errorf("storage: invalid category")
	}
	if reader == nil {
		return Asset{}, fmt.Errorf("storage: file content is required")
	}
	select {
	case <-ctx.Done():
		return Asset{}, ctx.Err()
	default:
	}

	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if !allowedImageType(contentType) {
		return Asset{}, ErrUnsupportedContentType
	}
	data, err := io.ReadAll(io.LimitReader(reader, s.maxBytes+1))
	if err != nil {
		return Asset{}, fmt.Errorf("read upload: %w", err)
	}
	if int64(len(data)) > s.maxBytes {
		return Asset{}, ErrFileTooLarge
	}
	if len(data) == 0 {
		return Asset{}, fmt.Errorf("storage: empty file")
	}
	if detected := http.DetectContentType(data); strings.HasPrefix(detected, "text/") || detected == "application/json" {
		return Asset{}, ErrUnsupportedContentType
	}

	identifier, err := randomIdentifier()
	if err != nil {
		return Asset{}, err
	}
	extension := extensionForContentType(contentType, filename)
	if err := os.MkdirAll(filepath.Join(s.rootDir, category), 0o755); err != nil {
		return Asset{}, fmt.Errorf("create upload directory: %w", err)
	}
	relativePath := filepath.Join(category, identifier+extension)
	absPath := filepath.Join(s.rootDir, relativePath)
	if err := os.WriteFile(absPath, data, 0o644); err != nil {
		return Asset{}, fmt.Errorf("write upload: %w", err)
	}
	digest := sha256.Sum256(data)
	objectKey := filepath.ToSlash(relativePath)
	return Asset{URL: s.publicPath + "/" + objectKey, Key: objectKey, SHA256: hex.EncodeToString(digest[:]), ContentType: contentType, Size: int64(len(data))}, nil
}

func (s *LocalFileStore) OpenURL(rawURL string) (io.ReadSeeker, string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", fmt.Errorf("storage: invalid asset url")
	}
	publicPrefix := strings.TrimRight(s.publicPath, "/") + "/"
	if !strings.HasPrefix(parsed.Path, publicPrefix) {
		return nil, "", fmt.Errorf("storage: asset url is outside storage")
	}
	relative := strings.TrimPrefix(parsed.Path, publicPrefix)
	relative = path.Clean("/" + relative)
	relative = strings.TrimPrefix(relative, "/")
	if relative == "" || relative == "." || strings.HasPrefix(relative, "../") || relative == ".." {
		return nil, "", fmt.Errorf("storage: invalid asset path")
	}
	file, err := os.Open(filepath.Join(s.rootDir, filepath.FromSlash(relative)))
	if err != nil {
		return nil, "", err
	}
	contentType := "application/octet-stream"
	if extensionType := mime.TypeByExtension(filepath.Ext(relative)); extensionType != "" {
		contentType = extensionType
	}
	return file, contentType, nil
}

func allowedImageType(value string) bool {
	return value == "image/jpeg" || value == "image/png" || value == "image/webp"
}

func extensionForContentType(contentType, filename string) string {
	switch contentType {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		if extension := strings.ToLower(filepath.Ext(filename)); extension == ".jpg" || extension == ".jpeg" {
			return extension
		}
		return ".jpg"
	}
}

func randomIdentifier() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate upload id: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
