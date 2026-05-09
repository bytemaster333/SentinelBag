package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const ttl = 1 * time.Hour

type Store struct {
	rdb *redis.Client
}

// NewStore connects to Redis and verifies the connection with a ping.
// Returns an error if the connection cannot be established.
func NewStore(redisURL string) (*Store, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("cache: parse redis url: %w", err)
	}

	rdb := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("cache: ping: %w", err)
	}

	return &Store{rdb: rdb}, nil
}

func cacheKey(tokenAddress string) string {
	return "sentinel:integrity:" + tokenAddress
}

// NewNoopStore returns a Store that silently skips all cache operations.
// Used when Redis is unavailable so callers need no nil checks.
func NewNoopStore() *Store {
	return &Store{rdb: nil}
}

// Get retrieves a cached value. Returns (nil, false) on any miss or error.
func (s *Store) Get(ctx context.Context, tokenAddress string) ([]byte, bool) {
	if s.rdb == nil {
		return nil, false
	}
	val, err := s.rdb.Get(ctx, cacheKey(tokenAddress)).Bytes()
	if err != nil {
		return nil, false
	}
	return val, true
}

// Set stores data with a 1-hour TTL. Errors are silently ignored (graceful degradation).
func (s *Store) Set(ctx context.Context, tokenAddress string, data []byte) {
	if s.rdb == nil {
		return
	}
	_ = s.rdb.SetEx(ctx, cacheKey(tokenAddress), data, ttl).Err()
}
