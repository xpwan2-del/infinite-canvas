package service

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/basketikun/infinite-canvas/config"
)

func TestSaveReferenceMediaR2Integration(t *testing.T) {
	if os.Getenv("RUN_R2_INTEGRATION") != "1" {
		t.Skip("set RUN_R2_INTEGRATION=1 to upload a small object to R2")
	}
	previous := config.Cfg
	t.Cleanup(func() { config.Cfg = previous })
	config.Cfg = config.Config{
		MediaStorageDriver:    "r2",
		R2Bucket:              os.Getenv("R2_BUCKET"),
		R2Endpoint:            os.Getenv("R2_ENDPOINT"),
		R2Region:              os.Getenv("R2_REGION"),
		R2PublicBaseURL:       os.Getenv("R2_PUBLIC_BASE_URL"),
		R2AccessKeyID:         os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey:     os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2TempReferencePrefix: "temp/reference",
	}
	result, err := SaveReferenceMedia(context.Background(), ReferenceMediaUploadInput{
		Reader:   strings.NewReader("top-ai-r2-reference-check"),
		MimeType: "text/plain",
		Ext:      ".txt",
		MaxBytes: 1024,
	})
	if err != nil {
		t.Fatalf("SaveReferenceMedia returned error: %v", err)
	}
	if !strings.HasPrefix(result.URL, strings.TrimRight(config.Cfg.R2PublicBaseURL, "/")+"/temp/reference/") {
		t.Fatalf("unexpected R2 URL: %q", result.URL)
	}
	response, err := http.Get(result.URL)
	if err != nil {
		t.Fatalf("R2 public URL is not readable: %v", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || string(body) != "top-ai-r2-reference-check" {
		t.Fatalf("unexpected R2 public response: status=%d body=%q", response.StatusCode, string(body))
	}
}
