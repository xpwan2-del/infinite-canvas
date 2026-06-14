package handler

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/basketikun/infinite-canvas/service"
)

const (
	referenceMediaMaxBytes    = 80 << 20
	referenceImageMaxBytes    = 30 << 20
	referenceVideoMaxBytes    = 50 << 20
	referenceAudioMaxBytes    = 15 << 20
	referenceImageAllowedText = "jpeg/png/webp/bmp/gif/heic/heif 图片"
	referenceVideoAllowedText = "mp4/mov 视频"
	referenceAudioAllowedText = "mp3/wav 音频"
	referenceMediaAllowedText = referenceImageAllowedText + "、" + referenceVideoAllowedText + "或" + referenceAudioAllowedText
)

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
