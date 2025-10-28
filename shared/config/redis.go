package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient holds the global Redis client instance
var RedisClient *redis.Client

// RedisMutex protects Redis operations during shutdown
var RedisMutex sync.RWMutex

// Ctx is the global context used for Redis operations
var Ctx = context.Background()

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Addr          string
	Password      string
	DB            int
	PoolSize      int
	MinIdleConns  int
	MaxRetries    int
	RetryDelay    time.Duration
	DialTimeout   time.Duration
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	PoolTimeout   time.Duration
	IdleTimeout   time.Duration
	MaxConnAge    time.Duration
	IdleCheckFreq time.Duration
}

// DefaultRedisConfig returns default Redis configuration
func DefaultRedisConfig() *RedisConfig {
	return &RedisConfig{
		Addr:          getEnv("REDIS_ADDR", "localhost:6379"),
		Password:      os.Getenv("REDIS_PASSWORD"),
		DB:            getEnvInt("REDIS_DB", 0),
		PoolSize:      getEnvInt("REDIS_POOL_SIZE", 10),
		MinIdleConns:  getEnvInt("REDIS_MIN_IDLE_CONNS", 5),
		MaxRetries:    getEnvInt("REDIS_MAX_RETRIES", 3),
		RetryDelay:    getEnvDuration("REDIS_RETRY_DELAY", 1*time.Second),
		DialTimeout:   getEnvDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
		ReadTimeout:   getEnvDuration("REDIS_READ_TIMEOUT", 3*time.Second),
		WriteTimeout:  getEnvDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
		PoolTimeout:   getEnvDuration("REDIS_POOL_TIMEOUT", 4*time.Second),
		IdleTimeout:   getEnvDuration("REDIS_IDLE_TIMEOUT", 5*time.Minute),
		MaxConnAge:    getEnvDuration("REDIS_MAX_CONN_AGE", 30*time.Minute),
		IdleCheckFreq: getEnvDuration("REDIS_IDLE_CHECK_FREQ", 1*time.Minute),
	}
}

// InitRedis initializes the Redis client using environment variables.
// Performs a Ping with a 2-second timeout to ensure Redis is reachable.
func InitRedis() {
	InitRedisWithConfig(DefaultRedisConfig())
}

// InitRedisWithConfig initializes Redis with custom configuration
func InitRedisWithConfig(config *RedisConfig) {
	RedisMutex.Lock()
	defer RedisMutex.Unlock()

	// Close existing client if any
	if RedisClient != nil {
		RedisClient.Close()
	}

	// Create new Redis client with enhanced configuration
	RedisClient = redis.NewClient(&redis.Options{
		Addr:            config.Addr,
		Password:        config.Password,
		DB:              config.DB,
		PoolSize:        config.PoolSize,
		MinIdleConns:    config.MinIdleConns,
		MaxRetries:      config.MaxRetries,
		MinRetryBackoff: config.RetryDelay,
		MaxRetryBackoff: config.RetryDelay * 3,
		DialTimeout:     config.DialTimeout,
		ReadTimeout:     config.ReadTimeout,
		WriteTimeout:    config.WriteTimeout,
		PoolTimeout:     config.PoolTimeout,
	})

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(Ctx, 5*time.Second)
	defer cancel()

	if err := RedisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	log.Printf("Redis connection established successfully: %s", config.Addr)

	// Start background health monitoring
	go monitorRedisHealth()
}

// GetRedisClient returns the Redis client with read lock
func GetRedisClient() *redis.Client {
	RedisMutex.RLock()
	defer RedisMutex.RUnlock()
	return RedisClient
}

// HealthCheck performs a Redis health check
func RedisHealthCheck() error {
	client := GetRedisClient()
	if client == nil {
		return fmt.Errorf("redis client is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	return nil
}

// GetRedisStats returns Redis connection statistics
func GetRedisStats() map[string]any {
	client := GetRedisClient()
	if client == nil {
		return map[string]any{"error": "redis client is nil"}
	}

	stats := client.PoolStats()
	return map[string]any{
		"hits":        stats.Hits,
		"misses":      stats.Misses,
		"timeouts":    stats.Timeouts,
		"total_conns": stats.TotalConns,
		"idle_conns":  stats.IdleConns,
		"stale_conns": stats.StaleConns,
	}
}

// CloseRedis closes the Redis connection gracefully
func CloseRedis() error {
	RedisMutex.Lock()
	defer RedisMutex.Unlock()

	if RedisClient == nil {
		return nil
	}

	log.Println("Closing Redis connections...")

	// Set a timeout for closing connections
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- RedisClient.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("Error closing Redis: %v", err)
			return err
		}
		log.Println("Redis connections closed successfully")
		RedisClient = nil
		return nil
	case <-ctx.Done():
		log.Println("Redis close timeout exceeded")
		RedisClient = nil // Force nil even on timeout
		return fmt.Errorf("redis close timeout")
	}
}

// Z creates a redis.Z object for sorted set operations.
func Z(postId uint, timeStamp int64) redis.Z {
	return redis.Z{
		Score:  float64(timeStamp), // used as the score to sort posts by creation time
		Member: postId,             // post id is the member value
	}
}

// RedisCache provides Redis caching operations with TTL and error handling
type RedisCache struct {
	client     *redis.Client
	defaultTTL time.Duration
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(ttl time.Duration) *RedisCache {
	return &RedisCache{
		client:     GetRedisClient(),
		defaultTTL: ttl,
	}
}

// Set stores a key-value pair with TTL
func (rc *RedisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if rc.client == nil {
		return fmt.Errorf("redis client is nil")
	}

	if ttl == 0 {
		ttl = rc.defaultTTL
	}

	return rc.client.Set(ctx, key, value, ttl).Err()
}

// Get retrieves a value by key
func (rc *RedisCache) Get(ctx context.Context, key string) (string, error) {
	if rc.client == nil {
		return "", fmt.Errorf("redis client is nil")
	}

	return rc.client.Get(ctx, key).Result()
}

// Del deletes one or more keys
func (rc *RedisCache) Del(ctx context.Context, keys ...string) error {
	if rc.client == nil {
		return fmt.Errorf("redis client is nil")
	}

	return rc.client.Del(ctx, keys...).Err()
}

// Exists checks if a key exists
func (rc *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	if rc.client == nil {
		return false, fmt.Errorf("redis client is nil")
	}

	result := rc.client.Exists(ctx, key)
	return result.Val() > 0, result.Err()
}

// Expire sets TTL for a key
func (rc *RedisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if rc.client == nil {
		return fmt.Errorf("redis client is nil")
	}

	return rc.client.Expire(ctx, key, ttl).Err()
}

// monitorRedisHealth runs background health monitoring
func monitorRedisHealth() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := RedisHealthCheck(); err != nil {
			log.Printf("Redis health check failed: %v", err)
		}
	}
}

// Helper functions to parse environment variables
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

// RedisLock provides distributed locking using Redis
type RedisLock struct {
	client *redis.Client
	key    string
	value  string
	ttl    time.Duration
}

// NewRedisLock creates a new Redis lock
func NewRedisLock(key, value string, ttl time.Duration) *RedisLock {
	return &RedisLock{
		client: GetRedisClient(),
		key:    key,
		value:  value,
		ttl:    ttl,
	}
}

// Acquire attempts to acquire the lock
func (rl *RedisLock) Acquire(ctx context.Context) (bool, error) {
	if rl.client == nil {
		return false, fmt.Errorf("redis client is nil")
	}

	result := rl.client.SetNX(ctx, rl.key, rl.value, rl.ttl)
	return result.Val(), result.Err()
}

// Release releases the lock
func (rl *RedisLock) Release(ctx context.Context) error {
	if rl.client == nil {
		return fmt.Errorf("redis client is nil")
	}

	// Lua script to ensure we only delete our own lock
	script := `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`

	result := rl.client.Eval(ctx, script, []string{rl.key}, rl.value)
	return result.Err()
}
