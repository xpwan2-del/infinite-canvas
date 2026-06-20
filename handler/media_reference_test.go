package handler

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/service"
)

func TestNormalizeReferenceMediaTypeSupportsAudio(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		ext         string
		wantMime    string
		wantExt     string
	}{
		{name: "mp3 mime", contentType: "audio/mpeg", ext: ".bin", wantMime: "audio/mpeg", wantExt: ".mp3"},
		{name: "wav ext fallback", contentType: "application/octet-stream", ext: ".wav", wantMime: "audio/wav", wantExt: ".wav"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mimeType, ext, ok := normalizeReferenceMediaType(tt.contentType, tt.ext)
			if !ok {
				t.Fatal("expected media type to be accepted")
			}
			if mimeType != tt.wantMime || ext != tt.wantExt {
				t.Fatalf("got (%q, %q), want (%q, %q)", mimeType, ext, tt.wantMime, tt.wantExt)
			}
		})
	}
}

func TestReferenceMediaTypeMaxBytes(t *testing.T) {
	if got := referenceMediaTypeMaxBytes("audio/mpeg"); got != referenceAudioMaxBytes {
		t.Fatalf("audio max bytes = %d, want %d", got, referenceAudioMaxBytes)
	}
	if got := referenceMediaTypeMaxBytes("video/mp4"); got != referenceVideoMaxBytes {
		t.Fatalf("video max bytes = %d, want %d", got, referenceVideoMaxBytes)
	}
	if got := referenceMediaTypeMaxBytes("image/png"); got != referenceImageMaxBytes {
		t.Fatalf("image max bytes = %d, want %d", got, referenceImageMaxBytes)
	}
}

func TestNormalizeGeneratedMediaTypeSupportsImagesAndVideos(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		ext         string
		wantMime    string
		wantExt     string
	}{
		{name: "png mime", contentType: "image/png", ext: ".bin", wantMime: "image/png", wantExt: ".png"},
		{name: "jpeg ext fallback", contentType: "application/octet-stream", ext: ".jpeg", wantMime: "image/jpeg", wantExt: ".jpg"},
		{name: "webp with charset", contentType: "image/webp; charset=binary", ext: "", wantMime: "image/webp", wantExt: ".webp"},
		{name: "mp4 mime", contentType: "video/mp4", ext: "", wantMime: "video/mp4", wantExt: ".mp4"},
		{name: "mov ext fallback", contentType: "", ext: ".mov", wantMime: "video/quicktime", wantExt: ".mov"},
		{name: "webm mime", contentType: "video/webm", ext: "", wantMime: "video/webm", wantExt: ".webm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mimeType, ext, ok := normalizeGeneratedMediaType(tt.contentType, tt.ext)
			if !ok {
				t.Fatal("expected generated media type to be accepted")
			}
			if mimeType != tt.wantMime || ext != tt.wantExt {
				t.Fatalf("got (%q, %q), want (%q, %q)", mimeType, ext, tt.wantMime, tt.wantExt)
			}
		})
	}
}

func TestNormalizeGeneratedMediaTypeRejectsAudioAndUnknown(t *testing.T) {
	if _, _, ok := normalizeGeneratedMediaType("audio/mpeg", ".mp3"); ok {
		t.Fatal("expected generated audio to be rejected")
	}
	if _, _, ok := normalizeGeneratedMediaType("application/octet-stream", ".txt"); ok {
		t.Fatal("expected unknown generated media type to be rejected")
	}
}

func TestGeneratedMediaTypeMaxBytes(t *testing.T) {
	if got := generatedMediaTypeMaxBytes("image/png"); got != generatedImageMaxBytes {
		t.Fatalf("generated image max bytes = %d, want %d", got, generatedImageMaxBytes)
	}
	if got := generatedMediaTypeMaxBytes("video/mp4"); got != generatedVideoMaxBytes {
		t.Fatalf("generated video max bytes = %d, want %d", got, generatedVideoMaxBytes)
	}
}

func TestGeneratedMediaSaveMessageUsesMediaKind(t *testing.T) {
	if got := generatedMediaSaveMessage(service.ErrReferenceMediaTooLarge, "image/png"); got != "生成图片超过大小限制，请使用 30MB 以内的图片" {
		t.Fatalf("image save message = %q", got)
	}
	if got := generatedMediaSaveMessage(service.ErrReferenceMediaStorageConfig, "video/mp4"); got != "生成视频存储未配置，请检查 R2 服务端环境变量" {
		t.Fatalf("video save message = %q", got)
	}
}

func TestGeneratedMediaHostAllowlist(t *testing.T) {
	previous := config.Cfg
	t.Cleanup(func() { config.Cfg = previous })
	config.Cfg = config.Config{GeneratedMediaAllowedHosts: "media.example.com,*.cdn.example.com"}

	if !isAllowedGeneratedMediaHost("media.example.com") {
		t.Fatal("expected exact host to be allowed")
	}
	if !isAllowedGeneratedMediaHost("video.cdn.example.com") {
		t.Fatal("expected subdomain host to be allowed")
	}
	if isAllowedGeneratedMediaHost("evil-example.com") {
		t.Fatal("expected unrelated host to be rejected")
	}
}

func TestSafeRemoteMediaURLRejectsNonHTTPSAndMissingAllowlist(t *testing.T) {
	previous := config.Cfg
	t.Cleanup(func() { config.Cfg = previous })
	config.Cfg = config.Config{GeneratedMediaAllowedHosts: "media.example.com"}

	if safeRemoteMediaURL(context.Background(), "http://media.example.com/video.mp4") {
		t.Fatal("expected http generated media URL to be rejected")
	}
	config.Cfg.GeneratedMediaAllowedHosts = ""
	if safeRemoteMediaURL(context.Background(), "https://media.example.com/video.mp4") {
		t.Fatal("expected generated media URL to be rejected without allowlist")
	}
}

func TestSafeLogMediaURLStripsQuery(t *testing.T) {
	got := safeLogMediaURL("https://media.example.com/generated/video.mp4?X-Amz-Signature=secret#token")
	if got != "https://media.example.com/generated/video.mp4" {
		t.Fatalf("safeLogMediaURL = %q", got)
	}
}

func TestReferenceMediaDirUsesAbsoluteSQLiteDataDir(t *testing.T) {
	previous := config.Cfg
	t.Cleanup(func() { config.Cfg = previous })
	root := t.TempDir()
	config.Cfg = config.Config{StorageDriver: "sqlite", DatabaseDSN: filepath.Join(root, "infinite-canvas.db")}

	if got := service.ReferenceMediaDir(); got != filepath.Join(root, "reference-media") {
		t.Fatalf("referenceMediaDir = %q", got)
	}
}
