package data

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Addr             string
	Username         string
	Password         string
	DB               int
	BucketCount      int
	BucketLockSize   int64
	OperationTimeout time.Duration
	BucketKeyTTL     time.Duration
}

type RedisBucketStore struct {
	client           *redis.Client
	bucketCount      int
	bucketLockSize   int64
	operationTimeout time.Duration
	bucketKeyTTL     time.Duration
}

func NewRedisBucketStore(ctx context.Context, cfg RedisConfig) (*RedisBucketStore, error) {
	if cfg.Addr == "" {
		return nil, nil
	}
	if cfg.BucketCount <= 0 {
		cfg.BucketCount = 16
	}
	if cfg.BucketLockSize <= 0 {
		cfg.BucketLockSize = 100
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 50 * time.Millisecond
	}
	if cfg.BucketKeyTTL <= 0 {
		cfg.BucketKeyTTL = 30 * time.Minute
	}
	client := redis.NewClient(&redis.Options{Addr: cfg.Addr, Username: cfg.Username, Password: cfg.Password, DB: cfg.DB})
	pingCtx, cancel := context.WithTimeout(ctx, cfg.OperationTimeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		client.Close()
		return nil, err
	}
	return &RedisBucketStore{client: client, bucketCount: cfg.BucketCount, bucketLockSize: cfg.BucketLockSize, operationTimeout: cfg.OperationTimeout, bucketKeyTTL: cfg.BucketKeyTTL}, nil
}

func (s *RedisBucketStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *RedisBucketStore) BucketCount() int      { return s.bucketCount }
func (s *RedisBucketStore) BucketLockSize() int64 { return s.bucketLockSize }

func (s *RedisBucketStore) Deduct(ctx context.Context, skuID int64, requestID string, quantity int64) (string, bool, error) {
	if s == nil || s.client == nil {
		return "", false, nil
	}
	start := stableBucket(requestID, s.bucketCount)
	for offset := 0; offset < s.bucketCount; offset++ {
		bucketNo := (start + offset) % s.bucketCount
		key := bucketStockKey(skuID, bucketNo)
		ok, err := s.deductBucket(ctx, key, quantity)
		if err != nil {
			return "", false, err
		}
		if ok {
			return key, true, nil
		}
	}
	return "", false, nil
}

func (s *RedisBucketStore) AddBucketStock(ctx context.Context, skuID int64, bucketNo int, quantity int64) (string, error) {
	if s == nil || s.client == nil || quantity <= 0 {
		return "", nil
	}
	key := bucketStockKey(skuID, bucketNo)
	ctx, cancel := context.WithTimeout(ctx, s.operationTimeout)
	defer cancel()
	pipe := s.client.TxPipeline()
	pipe.IncrBy(ctx, key, quantity)
	pipe.Expire(ctx, key, s.bucketKeyTTL)
	_, err := pipe.Exec(ctx)
	return key, err
}

func (s *RedisBucketStore) ReturnBucketStock(ctx context.Context, bucketKey string, quantity int64) error {
	if s == nil || s.client == nil || bucketKey == "" || quantity <= 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, s.operationTimeout)
	defer cancel()
	pipe := s.client.TxPipeline()
	pipe.IncrBy(ctx, bucketKey, quantity)
	pipe.Expire(ctx, bucketKey, s.bucketKeyTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisBucketStore) deductBucket(ctx context.Context, key string, quantity int64) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, s.operationTimeout)
	defer cancel()
	res, err := redisDeductScript.Run(ctx, s.client, []string{key}, quantity, s.bucketKeyTTL.Milliseconds()).Int()
	if err != nil {
		return false, err
	}
	return res >= 0, nil
}

var redisDeductScript = redis.NewScript(`
local current = redis.call('GET', KEYS[1])
if not current then
  return -1
end
current = tonumber(current)
local quantity = tonumber(ARGV[1])
if current < quantity then
  return -1
end
local remaining = redis.call('DECRBY', KEYS[1], quantity)
redis.call('PEXPIRE', KEYS[1], ARGV[2])
return remaining
`)

func bucketStockKey(skuID int64, bucketNo int) string {
	return fmt.Sprintf("inventory:{%d}:bucket:%d", skuID, bucketNo)
}

func stableBucket(requestID string, bucketCount int) int {
	if bucketCount <= 1 {
		return 0
	}
	sum := sha1.Sum([]byte(requestID))
	hexed := hex.EncodeToString(sum[:2])
	var n int
	_, _ = fmt.Sscanf(hexed, "%x", &n)
	return n % bucketCount
}
