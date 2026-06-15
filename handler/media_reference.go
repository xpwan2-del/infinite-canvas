package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/service"
)

const (
	referenceMediaMaxBytes    = 80 << 20
	referenceImageMaxBytes    = 30 << 20
	referenceVideoMaxBytes    = 50 << 20
	referenceAudioMaxBytes    = 15 << 20
	generatedVideoMaxBytes    = 120 << 20
	referenceImageAllowedText = "jpeg/png/webp/bmp/gif/heic/heif 图片"
	referenceVideoAllowedText = "mp4/mov 视频"
	referenceAudioAllowedText = "mp3/wav 音频"
	referenceMediaAllowedText = referenceImageAllowedText + "、" + referenceVideoAllowedText + "或" + referenceAudioAllowedText
)

type generatedMediaImportRequest struct {
	URL      string `json:"url"`
	MimeType string `json:"mimeType"`
}

func UploadReferenceMedia(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, referenceMediaMaxBytes+1)
	if err := r.ParseMultipartForm(referenceMediaMaxBytes); err != nil {
		Fail(w, "参考素材过大或上传格式不正确")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		Fail(w, "请上传参考图片或视频")
		return
	}
	defer file.Close()

	mimeType, ext, ok := normalizeReferenceMediaType(header.Header.Get("Content-Type"), filepath.Ext(header.Filename))
	if !ok {
		Fail(w, "参考素材格式不支持，请使用 "+referenceMediaAllowedText)
		return
	}
	result, err := service.SaveReferenceMedia(r.Context(), service.ReferenceMediaUploadInput{
		Reader:   file,
		MimeType: mimeType,
		Ext:      ext,
		MaxBytes: referenceMediaTypeMaxBytes(mimeType),
	})
	if err != nil {
		Fail(w, referenceMediaSaveMessage(err, mimeType))
		return
	}
	OK(w, result)
}

func ImportGeneratedMedia(w http.ResponseWriter, r *http.Request) {
	userID := "unknown"
	if user, ok := service.UserFromContext(r.Context()); ok {
		userID = user.ID
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var payload generatedMediaImportRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		Fail(w, "生成视频保存请求格式不正确")
		return
	}
	mediaURL := strings.TrimSpace(payload.URL)
	if !safeRemoteMediaURL(r.Context(), mediaURL) {
		log.Printf("blocked generated media import: user_id=%s source=%s", userID, safeLogMediaURL(mediaURL))
		Fail(w, "生成视频地址不安全或不可访问")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		Fail(w, "生成视频地址不正确")
		return
	}
	request.Header.Set("User-Agent", "TOP-AI-Canvas/1.0")
	client := &http.Client{
		Timeout:   90 * time.Second,
		Transport: safeRemoteMediaTransport(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 || !safeRemoteMediaURL(req.Context(), req.URL.String()) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	response, err := client.Do(request)
	if err != nil {
		log.Printf("import generated media failed: user_id=%s source=%s err=%v", userID, safeLogMediaURL(mediaURL), err)
		Fail(w, "生成视频下载失败")
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		Fail(w, "生成视频下载失败")
		return
	}
	contentType, ext, ok := normalizeGeneratedVideoType(response.Header.Get("Content-Type"), payload.MimeType, filepath.Ext(request.URL.Path))
	if !ok {
		Fail(w, "生成视频格式不支持，请使用 mp4/mov/webm 视频")
		return
	}
	result, err := service.SaveGeneratedMedia(r.Context(), service.ReferenceMediaUploadInput{
		Reader:   io.LimitReader(response.Body, generatedVideoMaxBytes+1),
		MimeType: contentType,
		Ext:      ext,
		MaxBytes: generatedVideoMaxBytes,
	})
	if err != nil {
		log.Printf("save imported generated media failed: user_id=%s source=%s err=%v", userID, safeLogMediaURL(mediaURL), err)
		Fail(w, generatedMediaSaveMessage(err))
		return
	}
	log.Printf("imported generated media: user_id=%s source_host=%s bytes=%d object_id=%s", userID, request.URL.Hostname(), result.Bytes, result.ID)
	OK(w, result)
}

func GeneratedMediaURL(w http.ResponseWriter, r *http.Request) {
	storageKey := strings.TrimSpace(r.URL.Query().Get("key"))
	mediaURL, err := service.GeneratedMediaURL(r.Context(), storageKey)
	if err != nil {
		Fail(w, "生成视频地址已失效或存储未配置")
		return
	}
	OK(w, map[string]string{"url": mediaURL})
}

func referenceMediaSaveMessage(err error, mimeType string) string {
	if errors.Is(err, service.ErrReferenceMediaEmpty) {
		return "参考素材为空"
	}
	if errors.Is(err, service.ErrReferenceMediaTooLarge) {
		return referenceMediaSizeMessage(mimeType)
	}
	if errors.Is(err, service.ErrReferenceMediaStorageConfig) {
		return "参考素材存储未配置，请检查 PUBLIC_BASE_URL 或 R2 服务端环境变量"
	}
	if errors.Is(err, service.ErrReferenceMediaUnsupportedDriver) {
		return "参考素材存储驱动不支持"
	}
	return "参考素材保存失败"
}

func ReferenceMedia(w http.ResponseWriter, r *http.Request, id string) {
	file, err := service.OpenLocalReferenceMedia(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	if mimeType := mimeTypeByReferenceMediaExt(filepath.Ext(id)); mimeType != "" {
		w.Header().Set("Content-Type", mimeType)
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(w, r, id, info.ModTime(), file)
}

func normalizeReferenceMediaType(contentType string, ext string) (string, string, bool) {
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	ext = strings.ToLower(strings.TrimSpace(ext))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = mimeTypeByReferenceMediaExt(ext)
	}
	if fixedExt := referenceMediaExtByMimeType(contentType); fixedExt != "" {
		return contentType, fixedExt, true
	}
	if mimeType := mimeTypeByReferenceMediaExt(ext); mimeType != "" {
		return mimeType, ext, true
	}
	return "", "", false
}

func normalizeGeneratedVideoType(contentTypes ...string) (string, string, bool) {
	for _, contentType := range contentTypes {
		contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
		switch contentType {
		case "video/mp4":
			return contentType, ".mp4", true
		case "video/quicktime", "video/mov":
			return "video/quicktime", ".mov", true
		case "video/webm":
			return contentType, ".webm", true
		case ".mp4":
			return "video/mp4", ".mp4", true
		case ".mov":
			return "video/quicktime", ".mov", true
		case ".webm":
			return "video/webm", ".webm", true
		}
	}
	return "", "", false
}

func referenceMediaExtByMimeType(mimeType string) string {
	switch strings.ToLower(mimeType) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	case "image/gif":
		return ".gif"
	case "image/heic":
		return ".heic"
	case "image/heif":
		return ".heif"
	case "video/mp4":
		return ".mp4"
	case "video/quicktime", "video/mov":
		return ".mov"
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/wav", "audio/x-wav", "audio/wave":
		return ".wav"
	default:
		return ""
	}
}

func mimeTypeByReferenceMediaExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".gif":
		return "image/gif"
	case ".heic":
		return "image/heic"
	case ".heif":
		return "image/heif"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	default:
		return ""
	}
}

func referenceMediaTypeMaxBytes(mimeType string) int64 {
	if strings.HasPrefix(mimeType, "image/") {
		return referenceImageMaxBytes
	}
	if strings.HasPrefix(mimeType, "video/") {
		return referenceVideoMaxBytes
	}
	if strings.HasPrefix(mimeType, "audio/") {
		return referenceAudioMaxBytes
	}
	return referenceMediaMaxBytes
}

func referenceMediaSizeMessage(mimeType string) string {
	if strings.HasPrefix(mimeType, "image/") {
		return "参考图片超过大小限制，请使用 30MB 以内的图片"
	}
	if strings.HasPrefix(mimeType, "video/") {
		return "参考视频超过大小限制，请使用 50MB 以内的 mp4/mov 视频"
	}
	if strings.HasPrefix(mimeType, "audio/") {
		return "参考音频超过大小限制，请使用 15MB 以内的 mp3/wav 音频"
	}
	return "参考素材超过大小限制"
}

func generatedMediaSaveMessage(err error) string {
	if errors.Is(err, service.ErrReferenceMediaEmpty) {
		return "生成视频为空"
	}
	if errors.Is(err, service.ErrReferenceMediaTooLarge) {
		return "生成视频超过大小限制，请联系管理员调整对象存储策略"
	}
	if errors.Is(err, service.ErrReferenceMediaStorageConfig) {
		return "生成视频存储未配置，请检查 R2 服务端环境变量"
	}
	if errors.Is(err, service.ErrReferenceMediaUnsupportedDriver) {
		return "生成视频存储驱动不支持"
	}
	return "生成视频保存失败"
}

func safeRemoteMediaURL(ctx context.Context, rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return false
	}
	if parsed.Scheme != "https" {
		return false
	}
	if !isAllowedGeneratedMediaHost(parsed.Hostname()) {
		return false
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", parsed.Hostname())
	if err != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if !isPublicIP(ip) {
			return false
		}
	}
	return true
}

func safeLogMediaURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return "invalid"
	}
	pathValue := parsed.EscapedPath()
	if len(pathValue) > 96 {
		pathValue = pathValue[:96] + "..."
	}
	return parsed.Scheme + "://" + parsed.Hostname() + pathValue
}

func safeRemoteMediaTransport() *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if !isAllowedGeneratedMediaHost(host) {
				return nil, fmt.Errorf("generated media host is not allowed: %s", host)
			}
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			var lastErr error
			dialer := &net.Dialer{Timeout: 15 * time.Second}
			for _, ip := range ips {
				if !isPublicIP(ip) {
					continue
				}
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, fmt.Errorf("generated media host has no public IP: %s", host)
		},
	}
}

func isAllowedGeneratedMediaHost(host string) bool {
	host = strings.Trim(strings.ToLower(host), ".")
	if host == "" {
		return false
	}
	for _, allowed := range splitHostAllowlist(config.Cfg.GeneratedMediaAllowedHosts) {
		if hostMatchesAllowedHost(host, allowed) {
			return true
		}
	}
	return false
}

func splitHostAllowlist(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.Trim(strings.ToLower(part), " .")
		value = strings.TrimPrefix(value, "*.")
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func hostMatchesAllowedHost(host string, allowed string) bool {
	return host == allowed || strings.HasSuffix(host, "."+allowed)
}

func isPublicIP(ip net.IP) bool {
	return ip != nil &&
		!ip.IsUnspecified() &&
		!ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsMulticast()
}
