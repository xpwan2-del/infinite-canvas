package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/basketikun/infinite-canvas/config"
)

type TopAIModels struct {
	Models      []string `json:"models"`
	TextModels  []string `json:"textModels"`
	ImageModels []string `json:"imageModels"`
	VideoModels []string `json:"videoModels"`
	AudioModels []string `json:"audioModels"`
}

type topAIModelsResponse struct {
	Data []topAIModelObject `json:"data"`
}

type topAIModelObject struct {
	ID string `json:"id"`
}

func FetchTopAIModels(ctx context.Context) (TopAIModels, error) {
	endpoint := topAIModelsEndpoint()
	if endpoint == "" {
		return TopAIModels{}, safeMessageError{message: "TOP-AI 模型接口未配置"}
	}
	apiKey := strings.TrimSpace(config.Cfg.TopAIGatewayAPIKey)
	if apiKey == "" {
		return TopAIModels{}, safeMessageError{message: "TOP-AI 网关凭证未配置"}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return TopAIModels{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("X-Canvas-Source", "infinite-canvas")
	response, err := topAIHTTPClient.Do(request)
	if err != nil {
		return TopAIModels{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return TopAIModels{}, fmt.Errorf("TOP-AI models status %d", response.StatusCode)
	}
	var payload topAIModelsResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return TopAIModels{}, err
	}
	models := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		models = append(models, item.ID)
	}
	return buildTopAIModels(uniqueModelNamesFold(models)), nil
}

func topAIModelsEndpoint() string {
	if endpoint := strings.TrimSpace(config.Cfg.TopAIModelsURL); endpoint != "" {
		return endpoint
	}
	base := firstNonEmpty(config.Cfg.TopAIInternalBaseURL, config.Cfg.TopAIPublicBaseURL)
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return ""
	}
	if !strings.HasSuffix(strings.ToLower(base), "/v1") {
		base += "/v1"
	}
	return base + "/models"
}

func buildTopAIModels(models []string) TopAIModels {
	return TopAIModels{
		Models:      models,
		TextModels:  filterTopAIModels(models, isTextModelName),
		ImageModels: filterTopAIModels(models, isImageModelName),
		VideoModels: filterTopAIModels(models, isVideoModelName),
		AudioModels: filterTopAIModels(models, isAudioModelName),
	}
}

func filterTopAIModels(models []string, predicate func(string) bool) []string {
	result := make([]string, 0, len(models))
	for _, modelName := range models {
		if predicate(modelName) {
			result = append(result, modelName)
		}
	}
	return result
}

func uniqueModelNamesFold(models []string) []string {
	result := make([]string, 0, len(models))
	seen := map[string]struct{}{}
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		key := strings.ToLower(modelName)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, modelName)
	}
	return result
}
