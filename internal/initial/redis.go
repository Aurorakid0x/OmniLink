package initial

import (
	"context"
	"fmt"
	"time"

	"OmniLink/internal/config"
	"OmniLink/pkg/redis"
	"OmniLink/pkg/zlog"

	goredis "github.com/redis/go-redis/v9"
)

func init() {
	conf := config.GetConfig()
	host := conf.RedisConfig.Host
	port := conf.RedisConfig.Port

	// 如果未配置主机，则跳过 Redis 初始化
	if host == "" {
		zlog.Info("Redis 未配置，跳过初始化")
		return
	}

	if port == 0 {
		port = 6379
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	zlog.Info(fmt.Sprintf("Redis connecting: %s", addr))

	client := goredis.NewClient(&goredis.Options{
		Addr:         addr,
		Password:     conf.RedisConfig.Password,
		DB:           conf.RedisConfig.DB,
		PoolSize:     conf.RedisConfig.PoolSize,
		MinIdleConns: conf.RedisConfig.MinIdleConns,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		zlog.Error(fmt.Sprintf("Redis 连接失败: %v", err))
		_ = client.Close()
		return
	}

	// 设置到 pkg/redis 包
	redis.SetClient(client)
	zlog.Info("Redis 连接成功")
}
