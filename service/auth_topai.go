package service

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/model"
	"github.com/basketikun/infinite-canvas/repository"
)

var topAIHTTPClient = &http.Client{Timeout: 8 * time.Second}

type topAIAPIResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Msg     string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

type topAISessionPayload struct {
	User topAIUser `json:"user"`
}

type topAIUser struct {
	ID          int64   `json:"id"`
	Email       string  `json:"email"`
	Username    string  `json:"username"`
	AvatarURL   string  `json:"avatar_url"`
	Balance     float64 `json:"balance"`
	Concurrency int     `json:"concurrency"`
	Status      string  `json:"status"`
}

func (payload topAIAPIResponse) user() (topAIUser, error) {
	var session topAISessionPayload
	if err := json.Unmarshal(payload.Data, &session); err == nil && session.User.ID > 0 {
		return session.User, nil
	}

	var user topAIUser
	if err := json.Unmarshal(payload.Data, &user); err == nil && user.ID > 0 {
		return user, nil
	}

	return topAIUser{}, safeMessageError{message: "TOP-AI 用户信息无效"}
}

func LoginWithTopAI(r *http.Request) (model.AuthSession, error) {
	profile, err := fetchTopAISession(r)
	if err != nil {
		return model.AuthSession{}, err
	}
	if profile.ID <= 0 {
		return model.AuthSession{}, safeMessageError{message: "TOP-AI 用户信息无效"}
	}
	if !strings.EqualFold(profile.Status, "active") {
		return model.AuthSession{}, safeMessageError{message: "TOP-AI 账号不可用"}
	}

	username := topAIUsername(profile.ID)
	user, ok, err := repository.GetUserByUsername(username)
	if err != nil {
		return model.AuthSession{}, err
	}
	if !ok {
		user = model.User{
			ID:        newID("user"),
			Username:  username,
			Role:      model.UserRoleUser,
			AffCode:   newAffCode(),
			Status:    model.UserStatusActive,
			CreatedAt: now(),
		}
	} else if user.Status == model.UserStatusBan {
		return model.AuthSession{}, safeMessageError{message: "账号已被禁用"}
	}

	user.Email = strings.TrimSpace(profile.Email)
	user.DisplayName = firstNonEmpty(profile.Username, profile.Email, username)
	user.AvatarURL = strings.TrimSpace(profile.AvatarURL)
	user.LastLoginAt = now()
	user.UpdatedAt = now()
	extra, _ := json.Marshal(map[string]any{
		"topAI": map[string]any{
			"id":          profile.ID,
			"email":       profile.Email,
			"username":    profile.Username,
			"balance":     profile.Balance,
			"concurrency": profile.Concurrency,
			"status":      profile.Status,
		},
	})
	user.Extra = string(extra)

	user, err = repository.SaveUser(user)
	if err != nil {
		return model.AuthSession{}, err
	}
	return newSession(user)
}

func fetchTopAISession(r *http.Request) (topAIUser, error) {
	endpoint, err := topAISessionEndpoint(r)
	if err != nil {
		return topAIUser{}, err
	}
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		return topAIUser{}, err
	}
	if authorization := strings.TrimSpace(r.Header.Get("Authorization")); authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	if cookie := strings.TrimSpace(r.Header.Get("Cookie")); cookie != "" {
		request.Header.Set("Cookie", cookie)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Canvas-Source", "infinite-canvas")

	response, err := topAIHTTPClient.Do(request)
	if err != nil {
		return topAIUser{}, safeMessageError{message: "TOP-AI 登录态校验失败"}
	}
	defer response.Body.Close()

	var payload topAIAPIResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return topAIUser{}, safeMessageError{message: "TOP-AI 登录态响应异常"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || payload.Code != 0 {
		message := firstNonEmpty(payload.Message, payload.Msg, "TOP-AI 登录态无效")
		return topAIUser{}, safeMessageError{message: message}
	}
	return payload.user()
}

func topAISessionEndpoint(r *http.Request) (string, error) {
	endpoint := strings.TrimSpace(config.Cfg.TopAISessionURL)
	if endpoint == "" {
		return "", safeMessageError{message: "TOP-AI 登录态接口未配置"}
	}
	if strings.HasPrefix(endpoint, "/") {
		return RequestOrigin(r) + endpoint, nil
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", safeMessageError{message: "TOP-AI 登录态接口配置无效"}
	}
	return endpoint, nil
}

func topAIUsername(id int64) string {
	return "topai-" + strconv.FormatInt(id, 10)
}
