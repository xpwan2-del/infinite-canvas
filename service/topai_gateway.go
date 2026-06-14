package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/model"
	"github.com/basketikun/infinite-canvas/repository"
	"github.com/google/uuid"
)

func BuildTopAIGatewayRequest(ctx context.Context, user model.AuthUser, method string, path string, body []byte, contentType string) (*http.Request, error) {
	apiKey := strings.TrimSpace(config.Cfg.TopAIGatewayAPIKey)
	if apiKey == "" {
		return nil, safeMessageError{message: "TOP-AI 网关凭证未配置"}
	}
	endpoint := topAIGatewayEndpoint(path)
	if endpoint == "" {
		return nil, safeMessageError{message: "TOP-AI 网关地址未配置"}
	}

	payload := body
	if strings.Contains(strings.ToLower(contentType), "application/json") {
		payload = injectCanvasMetadata(body, user)
	}
	var reader io.Reader
	if len(payload) > 0 {
		reader = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "infinite-canvas/source=canvas")
	request.Header.Set("X-Canvas-Source", "infinite-canvas")
	request.Header.Set("X-Client-Request-ID", "canvas-"+uuid.NewString())
	request.Header.Set("X-Top-AI-User-ID", topAIUserIDForCanvasUser(user))
	if contentType != "" && len(payload) > 0 {
		request.Header.Set("Content-Type", contentType)
	}
	return request, nil
}

func topAIGatewayEndpoint(path string) string {
	base := firstNonEmpty(config.Cfg.TopAIInternalBaseURL, config.Cfg.TopAIPublicBaseURL)
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return ""
	}
	if !strings.HasSuffix(strings.ToLower(base), "/v1") {
		base += "/v1"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func injectCanvasMetadata(body []byte, user model.AuthUser) []byte {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	metadata, _ := payload["metadata"].(map[string]any)
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["source"] = "canvas"
	metadata["canvas_user_id"] = user.ID
	metadata["top_ai_user_id"] = topAIUserIDForCanvasUser(user)
	payload["metadata"] = metadata
	next, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return next
}

func topAIUserIDForCanvasUser(user model.AuthUser) string {
	saved, ok, err := repository.GetUserByID(user.ID)
	if err != nil || !ok {
		return firstNonEmpty(user.Username, user.ID)
	}
	var extra struct {
		TopAI struct {
			ID any `json:"id"`
		} `json:"topAI"`
	}
	if err := json.Unmarshal([]byte(saved.Extra), &extra); err == nil && extra.TopAI.ID != nil {
		return strings.TrimSpace(fmt.Sprint(extra.TopAI.ID))
	}
	return firstNonEmpty(saved.Username, user.Username, user.ID)
}
