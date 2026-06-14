package service

import (
	"sync"
	"time"
)

type aiRateLimitEntry struct {
	windowStart time.Time
	count       int
}

var aiRateLimiter = struct {
	sync.Mutex
	hits map[string]aiRateLimitEntry
}{hits: map[string]aiRateLimitEntry{}}

func AllowAIRequest(userID string, limit int, window time.Duration) bool {
	if limit <= 0 || userID == "" {
		return true
	}
	now := time.Now()
	aiRateLimiter.Lock()
	defer aiRateLimiter.Unlock()

	entry := aiRateLimiter.hits[userID]
	if entry.windowStart.IsZero() || now.Sub(entry.windowStart) >= window {
		aiRateLimiter.hits[userID] = aiRateLimitEntry{windowStart: now, count: 1}
		return true
	}
	if entry.count >= limit {
		return false
	}
	entry.count++
	aiRateLimiter.hits[userID] = entry
	return true
}
