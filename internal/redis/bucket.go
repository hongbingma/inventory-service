package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// BucketManager Redis分桶管理器
type BucketManager struct {
	client      *redis.Client
	bucketCount int
}

// NewBucketManager 创建分桶管理器
func NewBucketManager(client *redis.Client, bucketCount int) *BucketManager {
	return &BucketManager{
		client:      client,
		bucketCount: bucketCount,
	}
}

// BucketKey 生成分桶key
func (m *BucketManager) BucketKey(invID string, bucketIndex int) string {
	return fmt.Sprintf("inv:bucket:%s:%d", invID, bucketIndex)
}

// BarrierKey 扣减屏障key(防止并发超卖)
func (m *BucketManager) BarrierKey(lockOrderID string) string {
	return fmt.Sprintf("inv:barrier:%s", lockOrderID)
}

// InitBucket 初始化分桶库存
func (m *BucketManager) InitBucket(ctx context.Context, invID string, bucketIndex int, quantity int, ttl time.Duration) error {
	key := m.BucketKey(invID, bucketIndex)
	err := m.client.Set(ctx, key, quantity, ttl).Err()
	if err != nil {
		return fmt.Errorf("init bucket failed: %w", err)
	}
	return nil
}

// DecrBucket 原子扣减分桶库存，返回剩余库存
func (m *BucketManager) DecrBucket(ctx context.Context, invID string, bucketIndex int, delta int) (int64, error) {
	key := m.BucketKey(invID, bucketIndex)
	// Lua脚本保证原子性：检查库存 >= delta 才扣减
	script := redis.NewScript(`
		local current = tonumber(redis.call('GET', KEYS[1]) or '0')
		if current >= tonumber(ARGV[1]) then
			local remain = redis.call('DECRBY', KEYS[1], ARGV[1])
			return remain
		else
			return -1
		end
	`)
	result, err := script.Run(ctx, m.client, []string{key}, delta).Int64()
	if err != nil {
		return 0, fmt.Errorf("decr bucket failed: %w", err)
	}
	if result < 0 {
		return 0, fmt.Errorf("insufficient bucket stock")
	}
	return result, nil
}

// IncrBucket 回补分桶库存
func (m *BucketManager) IncrBucket(ctx context.Context, invID string, bucketIndex int, delta int) (int64, error) {
	key := m.BucketKey(invID, bucketIndex)
	result, err := m.client.IncrBy(ctx, key, int64(delta)).Result()
	if err != nil {
		return 0, fmt.Errorf("incr bucket failed: %w", err)
	}
	return result, nil
}

// SetBarrier 设置扣减屏障
func (m *BucketManager) SetBarrier(ctx context.Context, lockOrderID string, ttl time.Duration) error {
	key := m.BarrierKey(lockOrderID)
	return m.client.Set(ctx, key, "1", ttl).Err()
}

// RemoveBarrier 移除扣减屏障
func (m *BucketManager) RemoveBarrier(ctx context.Context, lockOrderID string) error {
	key := m.BarrierKey(lockOrderID)
	return m.client.Del(ctx, key).Err()
}

// DeleteBucket 删除分桶
func (m *BucketManager) DeleteBucket(ctx context.Context, invID string, bucketIndex int) error {
	key := m.BucketKey(invID, bucketIndex)
	return m.client.Del(ctx, key).Err()
}
