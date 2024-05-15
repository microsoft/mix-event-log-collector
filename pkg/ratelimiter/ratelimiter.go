package ratelimiter

import (
	"time"

	"golang.org/x/time/rate"
)

type RateLimit struct {
	Limit float64 `json:"limit,omitempty"`
	Burst int     `json:"burst,omitempty"`
}

func NewRateLimiter(config RateLimit) *rate.Limiter {

	limit := config.Limit
	burst := config.Burst

	// calculate interval in ms to achieve the target limit per second
	interval := 1000.0 / float64(limit)
	rateLimiter := rate.NewLimiter(rate.Every(time.Duration(interval)*time.Millisecond), burst)

	return rateLimiter
}
