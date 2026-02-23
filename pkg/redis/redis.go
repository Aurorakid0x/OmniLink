package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var client *redis.Client

// SetClient 设置 Redis 客户端（由 internal/initial 调用）
func SetClient(c *redis.Client) {
	client = c
}

// Close 关闭 Redis 连接
func Close() error {
	if client == nil {
		return nil
	}
	return client.Close()
}

// IsConnected 检查 Redis 是否已连接
func IsConnected() bool {
	return client != nil
}

// GetClient 获取原始 Redis 客户端（高级用法）
func GetClient() *redis.Client {
	return client
}

// checkClient 检查客户端是否可用
func checkClient() error {
	if client == nil {
		return fmt.Errorf("Redis 未连接")
	}
	return nil
}

// ==================== String 操作 ====================

// Get 获取字符串值
func Get(ctx context.Context, key string) (string, error) {
	if err := checkClient(); err != nil {
		return "", err
	}
	return client.Get(ctx, key).Result()
}

// Set 设置字符串值
func Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if err := checkClient(); err != nil {
		return err
	}
	return client.Set(ctx, key, value, expiration).Err()
}

// SetNX 仅在 key 不存在时设置值（分布式锁常用）
func SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	if err := checkClient(); err != nil {
		return false, err
	}
	return client.SetNX(ctx, key, value, expiration).Result()
}

// Del 删除 key
func Del(ctx context.Context, keys ...string) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.Del(ctx, keys...).Result()
}

// Exists 检查 key 是否存在
func Exists(ctx context.Context, keys ...string) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.Exists(ctx, keys...).Result()
}

// Expire 设置过期时间
func Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	if err := checkClient(); err != nil {
		return false, err
	}
	return client.Expire(ctx, key, expiration).Result()
}

// TTL 获取剩余过期时间
func TTL(ctx context.Context, key string) (time.Duration, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.TTL(ctx, key).Result()
}

// Incr 原子自增
func Incr(ctx context.Context, key string) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.Incr(ctx, key).Result()
}

// IncrBy 原子自增指定数值
func IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.IncrBy(ctx, key, value).Result()
}

// Decr 原子自减
func Decr(ctx context.Context, key string) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.Decr(ctx, key).Result()
}

// DecrBy 原子自减指定数值
func DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.DecrBy(ctx, key, value).Result()
}

// ==================== Hash 操作 ====================

// HGet 获取 Hash 字段值
func HGet(ctx context.Context, key, field string) (string, error) {
	if err := checkClient(); err != nil {
		return "", err
	}
	return client.HGet(ctx, key, field).Result()
}

// HSet 设置 Hash 字段值
func HSet(ctx context.Context, key string, values ...interface{}) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.HSet(ctx, key, values...).Result()
}

// HGetAll 获取 Hash 所有字段和值
func HGetAll(ctx context.Context, key string) (map[string]string, error) {
	if err := checkClient(); err != nil {
		return nil, err
	}
	return client.HGetAll(ctx, key).Result()
}

// HDel 删除 Hash 字段
func HDel(ctx context.Context, key string, fields ...string) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.HDel(ctx, key, fields...).Result()
}

// HExists 检查 Hash 字段是否存在
func HExists(ctx context.Context, key, field string) (bool, error) {
	if err := checkClient(); err != nil {
		return false, err
	}
	return client.HExists(ctx, key, field).Result()
}

// HLen 获取 Hash 字段数量
func HLen(ctx context.Context, key string) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.HLen(ctx, key).Result()
}

// ==================== List 操作 ====================

// LPush 从列表左侧插入元素
func LPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.LPush(ctx, key, values...).Result()
}

// RPush 从列表右侧插入元素
func RPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.RPush(ctx, key, values...).Result()
}

// LPop 从列表左侧弹出元素
func LPop(ctx context.Context, key string) (string, error) {
	if err := checkClient(); err != nil {
		return "", err
	}
	return client.LPop(ctx, key).Result()
}

// RPop 从列表右侧弹出元素
func RPop(ctx context.Context, key string) (string, error) {
	if err := checkClient(); err != nil {
		return "", err
	}
	return client.RPop(ctx, key).Result()
}

// LRange 获取列表范围元素
func LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	if err := checkClient(); err != nil {
		return nil, err
	}
	return client.LRange(ctx, key, start, stop).Result()
}

// LLen 获取列表长度
func LLen(ctx context.Context, key string) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.LLen(ctx, key).Result()
}

// LRem 从列表中移除元素
func LRem(ctx context.Context, key string, count int64, value interface{}) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.LRem(ctx, key, count, value).Result()
}

// ==================== Set 操作 ====================

// SAdd 向集合添加元素
func SAdd(ctx context.Context, key string, members ...interface{}) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.SAdd(ctx, key, members...).Result()
}

// SRem 从集合移除元素
func SRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.SRem(ctx, key, members...).Result()
}

// SIsMember 检查元素是否在集合中
func SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	if err := checkClient(); err != nil {
		return false, err
	}
	return client.SIsMember(ctx, key, member).Result()
}

// SMembers 获取集合所有元素
func SMembers(ctx context.Context, key string) ([]string, error) {
	if err := checkClient(); err != nil {
		return nil, err
	}
	return client.SMembers(ctx, key).Result()
}

// SCard 获取集合元素数量
func SCard(ctx context.Context, key string) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.SCard(ctx, key).Result()
}

// ==================== Sorted Set 操作 ====================

// ZAdd 向有序集合添加元素
func ZAdd(ctx context.Context, key string, members ...redis.Z) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.ZAdd(ctx, key, members...).Result()
}

// ZRem 从有序集合移除元素
func ZRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.ZRem(ctx, key, members...).Result()
}

// ZRange 按分数从低到高获取元素
func ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	if err := checkClient(); err != nil {
		return nil, err
	}
	return client.ZRange(ctx, key, start, stop).Result()
}

// ZRevRange 按分数从高到低获取元素
func ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	if err := checkClient(); err != nil {
		return nil, err
	}
	return client.ZRevRange(ctx, key, start, stop).Result()
}

// ZScore 获取元素分数
func ZScore(ctx context.Context, key, member string) (float64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.ZScore(ctx, key, member).Result()
}

// ZRank 获取元素排名（从低到高）
func ZRank(ctx context.Context, key, member string) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.ZRank(ctx, key, member).Result()
}

// ZRevRank 获取元素排名（从高到低）
func ZRevRank(ctx context.Context, key, member string) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.ZRevRank(ctx, key, member).Result()
}

// ZCard 获取有序集合元素数量
func ZCard(ctx context.Context, key string) (int64, error) {
	if err := checkClient(); err != nil {
		return 0, err
	}
	return client.ZCard(ctx, key).Result()
}

// ==================== 分布式锁 ====================

// Lock 获取分布式锁
func Lock(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return SetNX(ctx, key, "1", expiration)
}

// Unlock 释放分布式锁
func Unlock(ctx context.Context, key string) error {
	_, err := Del(ctx, key)
	return err
}

// ==================== 批量操作 ====================

// MGet 批量获取
func MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	if err := checkClient(); err != nil {
		return nil, err
	}
	return client.MGet(ctx, keys...).Result()
}

// MSet 批量设置
func MSet(ctx context.Context, values ...interface{}) error {
	if err := checkClient(); err != nil {
		return err
	}
	return client.MSet(ctx, values...).Err()
}

// Pipeline 获取管道（用于批量操作）
func Pipeline() redis.Pipeliner {
	if client == nil {
		return nil
	}
	return client.Pipeline()
}

// ==================== 事务 ====================

// TxPipeline 获取事务管道
func TxPipeline() redis.Pipeliner {
	if client == nil {
		return nil
	}
	return client.TxPipeline()
}
