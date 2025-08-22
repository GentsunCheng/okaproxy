package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	
	"okaproxy/internal/config"
	"okaproxy/internal/logger"
)

// RedisManager manages Redis connections and operations
type RedisManager struct {
	client *redis.Client
	logger *logger.Logger
}

// NewRedisManager creates a new Redis manager
func NewRedisManager(logger *logger.Logger) *RedisManager {
	// Create Redis client with default options
	rdb := redis.NewClient(&redis.Options{
		Addr:            "localhost:6379",
		Password:        "",
		DB:              0,
		ConnMaxIdleTime: 10 * time.Second,
		MaxRetries:      3,
	})

	return &RedisManager{
		client: rdb,
		logger: logger,
	}
}

// Close closes the Redis connection
func (rm *RedisManager) Close() {
	if rm.client != nil {
		rm.client.Close()
	}
}

// Ping tests the Redis connection
func (rm *RedisManager) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	return rm.client.Ping(ctx).Err()
}

// RateLimitMiddleware creates a rate limiting middleware using Redis
func (rm *RedisManager) RateLimitMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip rate limiting if disabled
		if cfg.Limit.Count == 0 || cfg.Limit.Window == 0 {
			c.Next()
			return
		}

		clientIP := logger.GetClientIP(c.Request)
		
		// Create Redis key for this IP
		key := fmt.Sprintf("oka_rate_limit:%s", clientIP)
		
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		// Lua script for atomic increment with expiration
		luaScript := `
			local current
			current = redis.call("INCR", KEYS[1])
			if current == 1 then
				redis.call("EXPIRE", KEYS[1], ARGV[1])
			end
			return current
		`

		// Execute the Lua script
		result := rm.client.Eval(ctx, luaScript, []string{key}, cfg.Limit.Window)
		if result.Err() != nil {
			rm.logger.Errorf("Redis rate limit error: %v", result.Err())
			// Continue without rate limiting if Redis fails
			c.Next()
			return
		}

		requests, err := result.Int64()
		if err != nil {
			rm.logger.Errorf("Failed to parse rate limit result: %v", err)
			c.Next()
			return
		}

		// Check if rate limit exceeded
		if requests > int64(cfg.Limit.Count) {
			rm.logger.LogRateLimit(c.Request)
			
			c.JSON(http.StatusTooManyRequests, gin.H{
				"message": "Too many requests, please try again later.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// CacheMiddleware provides basic caching functionality
func (rm *RedisManager) CacheMiddleware(cacheDuration time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip caching for non-GET requests
		if c.Request.Method != "GET" {
			c.Next()
			return
		}

		// Create cache key
		key := fmt.Sprintf("cache:%s:%s", c.Request.Method, c.Request.URL.String())
		
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		// Try to get cached response
		cached := rm.client.Get(ctx, key)
		if cached.Err() == nil {
			// Cache hit
			if content := cached.Val(); content != "" {
				c.Header("X-Cache", "HIT")
				c.Data(http.StatusOK, "text/html", []byte(content))
				c.Abort()
				return
			}
		}

		// Continue with request processing
		c.Header("X-Cache", "MISS")
		c.Next()
	}
}

// SetCache stores a response in Redis cache
func (rm *RedisManager) SetCache(key string, value string, duration time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	
	return rm.client.Set(ctx, key, value, duration).Err()
}

// GetCache retrieves a cached value
func (rm *RedisManager) GetCache(key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	
	return rm.client.Get(ctx, key).Result()
}

// IncrementCounter increments a counter in Redis
func (rm *RedisManager) IncrementCounter(key string, expiration time.Duration) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Use pipeline for atomic operations
	pipe := rm.client.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, expiration)
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	
	return incrCmd.Val(), nil
}

// GetStats returns basic Redis stats
func (rm *RedisManager) GetStats() map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	stats := make(map[string]interface{})
	
	// Get Redis info
	info := rm.client.Info(ctx)
	if info.Err() == nil {
		stats["redis_info"] = "connected"
	} else {
		stats["redis_info"] = "error: " + info.Err().Error()
	}

	// Get pool stats
	poolStats := rm.client.PoolStats()
	stats["pool_stats"] = map[string]interface{}{
		"hits":         poolStats.Hits,
		"misses":       poolStats.Misses,
		"timeouts":     poolStats.Timeouts,
		"total_conns":  poolStats.TotalConns,
		"idle_conns":   poolStats.IdleConns,
		"stale_conns":  poolStats.StaleConns,
	}

	return stats
}