package platform

import (
	"github.com/redis/go-redis/v9"
)

func NewRedis(redisURL string) *redis.Client {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		opt = &redis.Options{Addr: "localhost:6379"}
	}
	return redis.NewClient(opt)
}
