package middleware

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/Mersad-Moghaddam/shared/config"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// RateLimitConfig defines rate limiting configuration
type RateLimitConfig struct {
	// Max requests per window
	MaxRequests int
	// Time window duration
	Window time.Duration
	// Key generator function
	KeyGenerator func(c *fiber.Ctx) string
	// Skip function to bypass rate limiting
	Skip func(c *fiber.Ctx) bool
	// Custom message for rate limit exceeded
	Message string
	// HTTP status code when rate limit is exceeded
	StatusCode int
	// Headers to include in response
	IncludeHeaders bool
	// Burst allowance (additional requests allowed temporarily)
	Burst int
	// Sliding window vs fixed window
	SlidingWindow bool
}

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	config      *RateLimitConfig
	redisClient *redis.Client
}

// DefaultRateLimitConfig returns default configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		MaxRequests:    100,
		Window:         time.Minute,
		KeyGenerator:   defaultKeyGenerator,
		Skip:           nil,
		Message:        "Rate limit exceeded",
		StatusCode:     429,
		IncludeHeaders: true,
		Burst:          10,
		SlidingWindow:  true,
	}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	return &RateLimiter{
		config:      config,
		redisClient: nil, // Will be set in RateLimitMiddleware
	}
}

// RateLimitMiddleware returns a rate limiting middleware
func (rl *RateLimiter) RateLimitMiddleware() fiber.Handler {
	// Set redis client here
	if rl.redisClient == nil {
		rl.redisClient = config.GetRedisClient()
	}

	return func(c *fiber.Ctx) error {
		// Skip rate limiting if skip function returns true
		if rl.config.Skip != nil && rl.config.Skip(c) {
			return c.Next()
		}

		// Generate key for this request
		key := rl.config.KeyGenerator(c)

		// Check rate limit
		allowed, remaining, resetTime, err := rl.checkRateLimit(key)
		if err != nil {
			log.Printf("Rate limit check failed: %v", err)
			// On error, allow the request but log the issue
			return c.Next()
		}

		// Add rate limit headers if enabled
		if rl.config.IncludeHeaders {
			c.Set("X-RateLimit-Limit", strconv.Itoa(rl.config.MaxRequests))
			c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			c.Set("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))
			c.Set("X-RateLimit-Window", rl.config.Window.String())
		}

		// If rate limit exceeded, return error
		if !allowed {
			c.Set("Retry-After", strconv.FormatInt(int64(time.Until(resetTime).Seconds()), 10))

			return c.Status(rl.config.StatusCode).JSON(fiber.Map{
				"error":       true,
				"message":     rl.config.Message,
				"code":        "RATE_LIMIT_EXCEEDED",
				"retry_after": int64(time.Until(resetTime).Seconds()),
				"limit":       rl.config.MaxRequests,
				"window":      rl.config.Window.String(),
			})
		}

		return c.Next()
	}
}

// checkRateLimit checks if the request is within rate limit
func (rl *RateLimiter) checkRateLimit(key string) (allowed bool, remaining int, resetTime time.Time, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	now := time.Now()

	if rl.config.SlidingWindow {
		return rl.checkSlidingWindow(ctx, key, now)
	}

	return rl.checkFixedWindow(ctx, key, now)
}

// checkSlidingWindow implements sliding window rate limiting
func (rl *RateLimiter) checkSlidingWindow(ctx context.Context, key string, now time.Time) (bool, int, time.Time, error) {
	pipe := rl.redisClient.Pipeline()

	// Remove old entries outside the time window
	windowStart := now.Add(-rl.config.Window)
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart.UnixNano(), 10))

	// Count current requests in window
	pipe.ZCard(ctx, key)

	// Add current request
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.UnixNano()),
		Member: fmt.Sprintf("%d-%d", now.UnixNano(), generateRequestID()),
	})

	// Set expiration
	pipe.Expire(ctx, key, rl.config.Window+time.Minute)

	results, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, now, fmt.Errorf("pipeline execution failed: %w", err)
	}

	// Get current count (before adding new request)
	currentCount := int(results[1].(*redis.IntCmd).Val())

	// Calculate remaining requests
	remaining := int(math.Max(float64(rl.config.MaxRequests-currentCount-1), 0))

	// Check if request is allowed (including burst)
	allowed := currentCount < (rl.config.MaxRequests + rl.config.Burst)

	// Calculate reset time (end of current window)
	resetTime := now.Add(rl.config.Window)

	return allowed, remaining, resetTime, nil
}

// checkFixedWindow implements fixed window rate limiting
func (rl *RateLimiter) checkFixedWindow(ctx context.Context, key string, now time.Time) (bool, int, time.Time, error) {
	// Calculate window key based on current time
	windowStart := now.Truncate(rl.config.Window)
	windowKey := fmt.Sprintf("%s:%d", key, windowStart.Unix())

	// Get current count and increment
	pipe := rl.redisClient.Pipeline()
	pipe.Incr(ctx, windowKey)
	pipe.Expire(ctx, windowKey, rl.config.Window+time.Minute)

	results, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, now, fmt.Errorf("pipeline execution failed: %w", err)
	}

	currentCount := int(results[0].(*redis.IntCmd).Val())

	// Calculate remaining requests
	remaining := int(math.Max(float64(rl.config.MaxRequests-currentCount-1), 0))

	// Check if request is allowed
	allowed := currentCount <= rl.config.MaxRequests

	// Calculate reset time (end of current window)
	resetTime := windowStart.Add(rl.config.Window)

	return allowed, remaining, resetTime, nil
}

// defaultKeyGenerator generates a key based on IP address
func defaultKeyGenerator(c *fiber.Ctx) string {
	return fmt.Sprintf("rate_limit:%s", c.IP())
}

// UserKeyGenerator generates a key based on user ID from JWT
func UserKeyGenerator(c *fiber.Ctx) string {
	userID := c.Locals("user_id")
	if userID != nil {
		return fmt.Sprintf("rate_limit:user:%v", userID)
	}
	return fmt.Sprintf("rate_limit:ip:%s", c.IP())
}

// EndpointKeyGenerator generates a key based on endpoint and IP
func EndpointKeyGenerator(c *fiber.Ctx) string {
	return fmt.Sprintf("rate_limit:%s:%s", c.Path(), c.IP())
}

// generateRateLimitRequestID generates a unique request ID for rate limiting
func generateRequestID() int64 {
	return time.Now().UnixNano()
}

// Multiple rate limiters for different scenarios

// GlobalRateLimit applies to all requests
func GlobalRateLimit(maxRequests int, window time.Duration) fiber.Handler {
	config := DefaultRateLimitConfig()
	config.MaxRequests = maxRequests
	config.Window = window
	config.KeyGenerator = func(c *fiber.Ctx) string {
		return "global_rate_limit"
	}

	limiter := NewRateLimiter(config)
	return limiter.RateLimitMiddleware()
}

// IPRateLimit applies per IP address
func IPRateLimit(maxRequests int, window time.Duration) fiber.Handler {
	config := DefaultRateLimitConfig()
	config.MaxRequests = maxRequests
	config.Window = window
	config.KeyGenerator = defaultKeyGenerator

	limiter := NewRateLimiter(config)
	return limiter.RateLimitMiddleware()
}

// UserRateLimit applies per authenticated user
func UserRateLimit(maxRequests int, window time.Duration) fiber.Handler {
	config := DefaultRateLimitConfig()
	config.MaxRequests = maxRequests
	config.Window = window
	config.KeyGenerator = UserKeyGenerator
	config.Skip = func(c *fiber.Ctx) bool {
		// Skip if user is not authenticated
		return c.Locals("user_id") == nil
	}

	limiter := NewRateLimiter(config)
	return limiter.RateLimitMiddleware()
}

// EndpointRateLimit applies per endpoint per IP
func EndpointRateLimit(maxRequests int, window time.Duration) fiber.Handler {
	config := DefaultRateLimitConfig()
	config.MaxRequests = maxRequests
	config.Window = window
	config.KeyGenerator = EndpointKeyGenerator

	limiter := NewRateLimiter(config)
	return limiter.RateLimitMiddleware()
}

// LoginRateLimit specifically for login endpoints
func LoginRateLimit() fiber.Handler {
	config := DefaultRateLimitConfig()
	config.MaxRequests = 5
	config.Window = 15 * time.Minute
	config.Message = "Too many login attempts, please try again later"
	config.KeyGenerator = func(c *fiber.Ctx) string {
		return fmt.Sprintf("login_rate_limit:%s", c.IP())
	}
	config.Burst = 0 // No burst for login attempts

	limiter := NewRateLimiter(config)
	return limiter.RateLimitMiddleware()
}

// APIRateLimit for API endpoints with different tiers
func APIRateLimit(tier string) fiber.Handler {
	configs := map[string]*RateLimitConfig{
		"basic": {
			MaxRequests: 1000,
			Window:      time.Hour,
		},
		"premium": {
			MaxRequests: 10000,
			Window:      time.Hour,
		},
		"unlimited": {
			MaxRequests: 1000000,
			Window:      time.Hour,
		},
	}

	config := DefaultRateLimitConfig()
	if tierConfig, exists := configs[tier]; exists {
		config.MaxRequests = tierConfig.MaxRequests
		config.Window = tierConfig.Window
	}

	config.KeyGenerator = func(c *fiber.Ctx) string {
		userID := c.Locals("user_id")
		if userID != nil {
			return fmt.Sprintf("api_rate_limit:%s:user:%v", tier, userID)
		}
		return fmt.Sprintf("api_rate_limit:%s:ip:%s", tier, c.IP())
	}

	limiter := NewRateLimiter(config)
	return limiter.RateLimitMiddleware()
}

// DynamicRateLimit adjusts limits based on user role or subscription
func DynamicRateLimit() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get user role from context
		userRole := c.Locals("user_role")

		var maxRequests int
		var window time.Duration

		switch role := userRole.(string); role {
		case "admin":
			maxRequests = 10000
			window = time.Hour
		case "premium":
			maxRequests = 5000
			window = time.Hour
		case "user":
			maxRequests = 1000
			window = time.Hour
		default:
			maxRequests = 100
			window = time.Hour
		}

		config := DefaultRateLimitConfig()
		config.MaxRequests = maxRequests
		config.Window = window
		config.KeyGenerator = UserKeyGenerator

		limiter := NewRateLimiter(config)
		return limiter.RateLimitMiddleware()(c)
	}
}

// GetRateLimitStatus returns current rate limit status for a key
func (rl *RateLimiter) GetRateLimitStatus(key string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	now := time.Now()

	if rl.config.SlidingWindow {
		windowStart := now.Add(-rl.config.Window)
		count, err := rl.redisClient.ZCount(ctx, key, strconv.FormatInt(windowStart.UnixNano(), 10), "+inf").Result()
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"current_requests": count,
			"limit":            rl.config.MaxRequests,
			"remaining":        rl.config.MaxRequests - int(count),
			"window_type":      "sliding",
			"window_duration":  rl.config.Window.String(),
			"reset_time":       now.Add(rl.config.Window).Unix(),
		}, nil
	}

	// Fixed window
	windowStart := now.Truncate(rl.config.Window)
	windowKey := fmt.Sprintf("%s:%d", key, windowStart.Unix())

	count, err := rl.redisClient.Get(ctx, windowKey).Int()
	if err == redis.Nil {
		count = 0
	} else if err != nil {
		return nil, err
	}

	return map[string]any{
		"current_requests": count,
		"limit":            rl.config.MaxRequests,
		"remaining":        rl.config.MaxRequests - count,
		"window_type":      "fixed",
		"window_duration":  rl.config.Window.String(),
		"reset_time":       windowStart.Add(rl.config.Window).Unix(),
	}, nil
}

// ClearRateLimit removes rate limit data for a key
func (rl *RateLimiter) ClearRateLimit(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if rl.config.SlidingWindow {
		return rl.redisClient.Del(ctx, key).Err()
	}

	// For fixed window, we need to clear all possible window keys
	now := time.Now()
	windowStart := now.Truncate(rl.config.Window)
	windowKey := fmt.Sprintf("%s:%d", key, windowStart.Unix())

	return rl.redisClient.Del(ctx, windowKey).Err()
}
