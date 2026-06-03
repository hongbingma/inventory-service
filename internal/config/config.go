package config

import (
	"os"
	"strconv"
)

// Config 全局配置
type Config struct {
	MySQLDSN     string
	RedisAddr    string
	RedisPass    string
	BucketCount  int     // Redis分桶数量
	LockPercent  float64 // 每次锁定比例
	MergeDelayMs int     // 合并提交延迟毫秒
}

func Load() *Config {
	bucketCount, _ := strconv.Atoi(getEnv("BUCKET_COUNT", "10"))
	lockPercent, _ := strconv.ParseFloat(getEnv("LOCK_PERCENT", "0.2"), 64)
	mergeDelayMs, _ := strconv.Atoi(getEnv("MERGE_DELAY_MS", "1000"))

	return &Config{
		MySQLDSN:     getEnv("MYSQL_DSN", "root:password@tcp(127.0.0.1:3306)/inventory?charset=utf8mb4&parseTime=True&loc=Local"),
		RedisAddr:    getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPass:    getEnv("REDIS_PASS", ""),
		BucketCount:  bucketCount,
		LockPercent:  lockPercent,
		MergeDelayMs: mergeDelayMs,
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
