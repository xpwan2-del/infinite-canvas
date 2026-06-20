package config

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	Port                       string `env:"PORT" envDefault:"8080"`
	JWTSecret                  string `env:"JWT_SECRET" envDefault:"infinite-canvas"`
	JWTExpireHours             int    `env:"JWT_EXPIRE_HOURS" envDefault:"168"`
	StorageDriver              string `env:"STORAGE_DRIVER" envDefault:"sqlite"`
	DatabaseDSN                string `env:"DATABASE_DSN" envDefault:"data/infinite-canvas.db"`
	PublicBaseURL              string `env:"PUBLIC_BASE_URL"`
	MediaStorageDriver         string `env:"MEDIA_STORAGE_DRIVER" envDefault:"local"`
	AIRequestMaxBytes          int64  `env:"AI_REQUEST_MAX_BYTES" envDefault:"83886080"`
	AIUserRateLimit            int    `env:"AI_USER_RATE_LIMIT" envDefault:"30"`
	CanvasDisableLocalCredits  bool   `env:"CANVAS_DISABLE_LOCAL_CREDITS" envDefault:"false"`
	CanvasForceTopAIGateway    bool   `env:"CANVAS_FORCE_TOP_AI_GATEWAY" envDefault:"false"`
	TopAIPublicBaseURL         string `env:"TOP_AI_PUBLIC_BASE_URL"`
	TopAIInternalBaseURL       string `env:"TOP_AI_INTERNAL_BASE_URL"`
	TopAIModelsURL             string `env:"TOP_AI_MODELS_URL"`
	TopAIGatewayAPIKey         string `env:"TOP_AI_GATEWAY_API_KEY"`
	R2Bucket                   string `env:"R2_BUCKET"`
	R2Endpoint                 string `env:"R2_ENDPOINT"`
	R2Region                   string `env:"R2_REGION" envDefault:"auto"`
	R2PublicBaseURL            string `env:"R2_PUBLIC_BASE_URL"`
	R2AccessKeyID              string `env:"R2_ACCESS_KEY_ID"`
	R2SecretAccessKey          string `env:"R2_SECRET_ACCESS_KEY"`
	R2TempReferencePrefix      string `env:"R2_TEMP_REFERENCE_PREFIX" envDefault:"temp/reference"`
	R2GeneratedPrefix          string `env:"R2_GENERATED_PREFIX" envDefault:"generated"`
	R2GeneratedSignedURLTTL    int64  `env:"R2_GENERATED_SIGNED_URL_TTL_SECONDS" envDefault:"604800"`
	GeneratedMediaAllowedHosts string `env:"GENERATED_MEDIA_ALLOWED_HOSTS"`
	TopAISessionURL            string `env:"TOP_AI_SESSION_URL" envDefault:"/api/v1/app/canvas/session"`
}

var Cfg Config

func Load() error {
	_ = godotenv.Load()
	if err := env.Parse(&Cfg); err != nil {
		return err
	}
	normalizeDockerSQLiteDSN("/app/data")
	if strings.TrimSpace(Cfg.JWTSecret) == "" || Cfg.JWTSecret == "infinite-canvas" {
		secret, err := randomSecret()
		if err != nil {
			return err
		}
		Cfg.JWTSecret = secret
	}
	return nil
}

func normalizeDockerSQLiteDSN(appDataDir string) {
	driver := strings.ToLower(strings.TrimSpace(Cfg.StorageDriver))
	if driver != "" && driver != "sqlite" {
		return
	}
	dsn := strings.TrimSpace(Cfg.DatabaseDSN)
	if dsn == "" || dsn == ":memory:" || strings.HasPrefix(dsn, "file:") {
		return
	}
	pathPart, suffix := dsn, ""
	if index := strings.Index(dsn, "?"); index >= 0 {
		pathPart = dsn[:index]
		suffix = dsn[index:]
	}
	if filepath.IsAbs(pathPart) {
		return
	}
	slashPath := filepath.ToSlash(pathPart)
	if slashPath != "data" && !strings.HasPrefix(slashPath, "data/") {
		return
	}
	if _, err := os.Stat(appDataDir); err != nil {
		return
	}
	Cfg.DatabaseDSN = filepath.Join(filepath.Dir(appDataDir), filepath.FromSlash(slashPath)) + suffix
}

func randomSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
