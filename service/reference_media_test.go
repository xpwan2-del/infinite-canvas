package service

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/basketikun/infinite-canvas/config"
)

func TestSaveReferenceMediaLocal(t *testing.T) {
	previous := config.Cfg
	t.Cleanup(func() { config.Cfg = previous })
	root := t.TempDir()
	config.Cfg = config.Config{
		StorageDriver:      "sqlite",
		DatabaseDSN:        filepath.Join(root, "infinite-canvas.db"),
		PublicBaseURL:      "https://canvas.example.com/",
		MediaStorageDriver: "local",
	}

	result, err := SaveReferenceMedia(context.Background(), ReferenceMediaUploadInput{
		Reader:   strings.NewReader("image-bytes"),
		MimeType: "image/png",
		Ext:      ".png",
		MaxBytes: 32,
	})
	if err != nil {
		t.Fatalf("SaveReferenceMedia returned error: %v", err)
	}
	if result.ID == "" || !strings.HasSuffix(result.ID, ".png") {
		t.Fatalf("unexpected id: %q", result.ID)
	}
	if result.URL != "https://canvas.example.com/api/media/references/"+result.ID {
		t.Fatalf("unexpected url: %q", result.URL)
	}
	if result.MimeType != "image/png" || result.Bytes != int64(len("image-bytes")) {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSaveReferenceMediaRejectsTooLargeLocalFile(t *testing.T) {
	previous := config.Cfg
	t.Cleanup(func() { config.Cfg = previous })
	root := t.TempDir()
	config.Cfg = config.Config{
		StorageDriver:      "sqlite",
		DatabaseDSN:        filepath.Join(root, "infinite-canvas.db"),
		PublicBaseURL:      "https://canvas.example.com",
		MediaStorageDriver: "local",
	}

	_, err := SaveReferenceMedia(context.Background(), ReferenceMediaUploadInput{
		Reader:   strings.NewReader("too-large"),
		MimeType: "video/mp4",
		Ext:      ".mp4",
		MaxBytes: 3,
	})
	if !errors.Is(err, ErrReferenceMediaTooLarge) {
		t.Fatalf("error = %v, want ErrReferenceMediaTooLarge", err)
	}
}

func TestJoinPublicURL(t *testing.T) {
	if got := joinPublicURL("https://media.example.com/", "temp/reference/a b.mp4"); got != "https://media.example.com/temp/reference/a%20b.mp4" {
		t.Fatalf("joinPublicURL = %q", got)
	}
}
