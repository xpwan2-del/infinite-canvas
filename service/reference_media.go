package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/basketikun/infinite-canvas/config"
	"github.com/google/uuid"
)

var (
	ErrReferenceMediaEmpty             = errors.New("reference media is empty")
	ErrReferenceMediaTooLarge          = errors.New("reference media is too large")
	ErrReferenceMediaStorageConfig     = errors.New("reference media storage is not configured")
	ErrReferenceMediaUnsupportedDriver = errors.New("reference media storage driver is unsupported")
)

type ReferenceMediaUploadInput struct {
	Reader   io.Reader
	MimeType string
	Ext      string
	MaxBytes int64
}

type ReferenceMediaUploadResult struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	MimeType string `json:"mimeType"`
	Bytes    int64  `json:"bytes"`
}

func SaveReferenceMedia(ctx context.Context, input ReferenceMediaUploadInput) (ReferenceMediaUploadResult, error) {
	if input.Reader == nil || input.Ext == "" || input.MimeType == "" || input.MaxBytes <= 0 {
		return ReferenceMediaUploadResult{}, ErrReferenceMediaStorageConfig
	}
	driver := strings.ToLower(strings.TrimSpace(config.Cfg.MediaStorageDriver))
	if driver == "" {
		driver = "local"
	}
	id := uuid.NewString() + input.Ext
	switch driver {
	case "local":
		return saveLocalReferenceMedia(id, input)
	case "r2", "s3":
		return saveR2ReferenceMedia(ctx, id, input)
	default:
		return ReferenceMediaUploadResult{}, ErrReferenceMediaUnsupportedDriver
	}
}

func OpenLocalReferenceMedia(id string) (*os.File, error) {
	if id == "" || id != filepath.Base(id) || strings.Contains(id, "..") {
		return nil, os.ErrNotExist
	}
	return os.Open(filepath.Join(ReferenceMediaDir(), id))
}

func ReferenceMediaDir() string {
	return filepath.Join(referenceDataDir(), "reference-media")
}

func saveLocalReferenceMedia(id string, input ReferenceMediaUploadInput) (ReferenceMediaUploadResult, error) {
	publicBaseURL := strings.TrimRight(strings.TrimSpace(config.Cfg.PublicBaseURL), "/")
	if publicBaseURL == "" {
		return ReferenceMediaUploadResult{}, ErrReferenceMediaStorageConfig
	}
	if err := os.MkdirAll(ReferenceMediaDir(), 0o755); err != nil {
		return ReferenceMediaUploadResult{}, err
	}
	targetPath := filepath.Join(ReferenceMediaDir(), id)
	target, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return ReferenceMediaUploadResult{}, err
	}
	bytes, copyErr := copyLimited(target, input.Reader, input.MaxBytes)
	closeErr := target.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(targetPath)
		if copyErr != nil {
			return ReferenceMediaUploadResult{}, copyErr
		}
		return ReferenceMediaUploadResult{}, closeErr
	}
	if bytes <= 0 {
		_ = os.Remove(targetPath)
		return ReferenceMediaUploadResult{}, ErrReferenceMediaEmpty
	}
	return ReferenceMediaUploadResult{
		ID:       id,
		URL:      fmt.Sprintf("%s/api/media/references/%s", publicBaseURL, id),
		MimeType: input.MimeType,
		Bytes:    bytes,
	}, nil
}

func saveR2ReferenceMedia(ctx context.Context, id string, input ReferenceMediaUploadInput) (ReferenceMediaUploadResult, error) {
	bucket := strings.TrimSpace(config.Cfg.R2Bucket)
	endpoint := strings.TrimRight(strings.TrimSpace(config.Cfg.R2Endpoint), "/")
	publicBaseURL := strings.TrimRight(strings.TrimSpace(config.Cfg.R2PublicBaseURL), "/")
	accessKeyID := strings.TrimSpace(config.Cfg.R2AccessKeyID)
	secretAccessKey := strings.TrimSpace(config.Cfg.R2SecretAccessKey)
	if bucket == "" || endpoint == "" || publicBaseURL == "" || accessKeyID == "" || secretAccessKey == "" {
		return ReferenceMediaUploadResult{}, ErrReferenceMediaStorageConfig
	}
	tempFile, bytes, err := spoolLimited(input.Reader, input.MaxBytes)
	if err != nil {
		return ReferenceMediaUploadResult{}, err
	}
	defer func() {
		name := tempFile.Name()
		_ = tempFile.Close()
		_ = os.Remove(name)
	}()
	if bytes <= 0 {
		return ReferenceMediaUploadResult{}, ErrReferenceMediaEmpty
	}
	key := path.Join(cleanR2Prefix(config.Cfg.R2TempReferencePrefix), id)
	region := strings.TrimSpace(config.Cfg.R2Region)
	if region == "" {
		region = "auto"
	}
	client := s3.NewFromConfig(aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			if service == s3.ServiceID {
				return aws.Endpoint{URL: endpoint, SigningRegion: region}, nil
			}
			return aws.Endpoint{}, fmt.Errorf("unknown endpoint for %s", service)
		}),
	}, func(options *s3.Options) {
		options.UsePathStyle = true
	})
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        tempFile,
		ContentType: aws.String(input.MimeType),
	}); err != nil {
		return ReferenceMediaUploadResult{}, err
	}
	return ReferenceMediaUploadResult{
		ID:       id,
		URL:      joinPublicURL(publicBaseURL, key),
		MimeType: input.MimeType,
		Bytes:    bytes,
	}, nil
}

func copyLimited(dst io.Writer, src io.Reader, maxBytes int64) (int64, error) {
	limited := io.LimitReader(src, maxBytes+1)
	bytes, err := io.Copy(dst, limited)
	if err != nil {
		return bytes, err
	}
	if bytes > maxBytes {
		return bytes, ErrReferenceMediaTooLarge
	}
	return bytes, nil
}

func spoolLimited(src io.Reader, maxBytes int64) (*os.File, int64, error) {
	tempFile, err := os.CreateTemp("", "reference-media-*")
	if err != nil {
		return nil, 0, err
	}
	bytes, err := copyLimited(tempFile, src, maxBytes)
	if err != nil {
		name := tempFile.Name()
		_ = tempFile.Close()
		_ = os.Remove(name)
		return nil, bytes, err
	}
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		name := tempFile.Name()
		_ = tempFile.Close()
		_ = os.Remove(name)
		return nil, bytes, err
	}
	return tempFile, bytes, nil
}

func cleanR2Prefix(prefix string) string {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return "temp/reference"
	}
	return prefix
}

func joinPublicURL(baseURL string, key string) string {
	joined, err := url.JoinPath(baseURL, key)
	if err == nil {
		return joined
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(key, "/")
}

func referenceDataDir() string {
	driver := strings.ToLower(strings.TrimSpace(config.Cfg.StorageDriver))
	dsn := strings.TrimSpace(config.Cfg.DatabaseDSN)
	if (driver == "" || driver == "sqlite") && dsn != "" && dsn != ":memory:" && !strings.HasPrefix(dsn, "file:") {
		pathPart := dsn
		if index := strings.Index(dsn, "?"); index >= 0 {
			pathPart = dsn[:index]
		}
		if filepath.IsAbs(pathPart) {
			return filepath.Dir(pathPart)
		}
	}
	if _, err := os.Stat("/app/data"); err == nil {
		return "/app/data"
	}
	return "data"
}
