package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// S3Config configures an S3-compatible private object store. It also works
// with MinIO and most cloud providers that implement the S3 API.
type S3Config struct {
	Endpoint   string
	Bucket     string
	Region     string
	AccessKey  string
	SecretKey  string
	PathStyle  bool
	PublicPath string
	MaxBytes   int64
}

// S3Store keeps objects private. API responses expose an internal /uploads
// key; protectedUploadHandler verifies the application signature and this
// store then reads the object through a signed S3 request.
type S3Store struct {
	endpoint   *url.URL
	bucket     string
	region     string
	accessKey  string
	secretKey  string
	pathStyle  bool
	publicPath string
	maxBytes   int64
	http       *http.Client
}

func NewS3Store(cfg S3Config) (*S3Store, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" || strings.TrimSpace(cfg.Bucket) == "" || strings.TrimSpace(cfg.Region) == "" {
		return nil, fmt.Errorf("storage s3 endpoint, bucket and region are required")
	}
	if strings.TrimSpace(cfg.AccessKey) == "" || strings.TrimSpace(cfg.SecretKey) == "" {
		return nil, fmt.Errorf("storage s3 access key and secret key are required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("storage s3 endpoint is invalid")
	}
	publicPath := strings.TrimRight(strings.TrimSpace(cfg.PublicPath), "/")
	if publicPath == "" {
		publicPath = "/uploads"
	}
	maxBytes := cfg.MaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxPhotoBytes
	}
	return &S3Store{
		endpoint:   parsed,
		bucket:     strings.TrimSpace(cfg.Bucket),
		region:     strings.TrimSpace(cfg.Region),
		accessKey:  strings.TrimSpace(cfg.AccessKey),
		secretKey:  strings.TrimSpace(cfg.SecretKey),
		pathStyle:  cfg.PathStyle,
		publicPath: publicPath,
		maxBytes:   maxBytes,
		http:       &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (s *S3Store) Save(ctx context.Context, filename, contentType string, reader io.Reader) (Asset, error) {
	return s.SaveIn(ctx, "pickup", filename, contentType, reader)
}

func (s *S3Store) SaveIn(ctx context.Context, category, filename, contentType string, reader io.Reader) (Asset, error) {
	category = strings.TrimSpace(category)
	if category == "" || category == "." || category == ".." || path.Base(category) != category {
		return Asset{}, fmt.Errorf("storage: invalid category")
	}
	if reader == nil {
		return Asset{}, fmt.Errorf("storage: file content is required")
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
	objectKey := path.Join(category, identifier+extensionForContentType(contentType, filename))
	if err := s.do(ctx, http.MethodPut, objectKey, contentType, data); err != nil {
		return Asset{}, err
	}
	digest := sha256.Sum256(data)
	return Asset{
		URL:         s.publicPath + "/" + objectKey,
		Key:         objectKey,
		SHA256:      hex.EncodeToString(digest[:]),
		ContentType: contentType,
		Size:        int64(len(data)),
	}, nil
}

func (s *S3Store) OpenURL(rawURL string) (io.ReadSeeker, string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", fmt.Errorf("storage: invalid asset url")
	}
	prefix := strings.TrimRight(s.publicPath, "/") + "/"
	if !strings.HasPrefix(parsed.Path, prefix) {
		return nil, "", fmt.Errorf("storage: asset url is outside storage")
	}
	objectKey := strings.TrimPrefix(parsed.Path, prefix)
	cleanKey := path.Clean("/" + objectKey)
	if objectKey == "" || cleanKey != "/"+objectKey || strings.HasPrefix(objectKey, "../") {
		return nil, "", fmt.Errorf("storage: invalid asset path")
	}
	data, contentType, err := s.get(context.Background(), objectKey)
	if err != nil {
		return nil, "", err
	}
	return bytes.NewReader(data), contentType, nil
}

func (s *S3Store) get(ctx context.Context, objectKey string) ([]byte, string, error) {
	request, err := s.newRequest(ctx, http.MethodGet, objectKey, "", nil)
	if err != nil {
		return nil, "", err
	}
	if err := signAWSRequest(request, s.region, s.accessKey, s.secretKey, sha256Hex(nil)); err != nil {
		return nil, "", err
	}
	resp, err := s.http.Do(request)
	if err != nil {
		return nil, "", fmt.Errorf("storage s3 read object: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("storage s3 read object returned http %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, s.maxBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("storage s3 read object body: %w", err)
	}
	if int64(len(data)) > s.maxBytes {
		return nil, "", ErrFileTooLarge
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = mime.TypeByExtension(path.Ext(objectKey))
	}
	return data, contentType, nil
}

func (s *S3Store) do(ctx context.Context, method, objectKey, contentType string, body []byte) error {
	request, err := s.newRequest(ctx, method, objectKey, contentType, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if err := signAWSRequest(request, s.region, s.accessKey, s.secretKey, sha256Hex(body)); err != nil {
		return err
	}
	resp, err := s.http.Do(request)
	if err != nil {
		return fmt.Errorf("storage s3 write object: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("storage s3 write object returned http %d", resp.StatusCode)
	}
	return nil
}

func (s *S3Store) newRequest(ctx context.Context, method, objectKey, contentType string, body io.Reader) (*http.Request, error) {
	objectURL := s.objectURL(objectKey)
	request, err := http.NewRequestWithContext(ctx, method, objectURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("storage s3 create request: %w", err)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return request, nil
}

func (s *S3Store) objectURL(objectKey string) *url.URL {
	result := *s.endpoint
	if s.pathStyle {
		result.Path = joinURLPath(s.endpoint.Path, s.bucket, objectKey)
	} else {
		result.Host = s.bucket + "." + s.endpoint.Host
		result.Path = joinURLPath(s.endpoint.Path, objectKey)
	}
	result.RawQuery = ""
	return &result
}

func joinURLPath(base string, parts ...string) string {
	segments := make([]string, 0, len(parts)+1)
	if strings.Trim(base, "/") != "" {
		segments = append(segments, strings.Trim(base, "/"))
	}
	for _, part := range parts {
		segments = append(segments, strings.Trim(part, "/"))
	}
	return "/" + path.Join(segments...)
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func signAWSRequest(request *http.Request, region, accessKey, secretKey, payloadHash string) error {
	now := time.Now().UTC()
	date := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	request.Header.Set("Host", request.URL.Host)
	request.Header.Set("X-Amz-Date", amzDate)
	request.Header.Set("X-Amz-Content-Sha256", payloadHash)
	canonicalHeaders := "host:" + request.URL.Host + "\n" + "x-amz-content-sha256:" + payloadHash + "\n" + "x-amz-date:" + amzDate + "\n"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalQuery := request.URL.Query().Encode()
	canonicalURI := request.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalRequest := strings.Join([]string{request.Method, canonicalURI, canonicalQuery, canonicalHeaders, signedHeaders, payloadHash}, "\n")
	scope := date + "/" + region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{"AWS4-HMAC-SHA256", amzDate, scope, sha256Hex([]byte(canonicalRequest))}, "\n")
	kDate := hmacSHA256([]byte("AWS4"+secretKey), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, "s3")
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))
	request.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+accessKey+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+signature)
	return nil
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}
