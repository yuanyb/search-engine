package db

import (
	"github.com/go-redis/redis/v8"
	"search-engine/index/config"
)

var Redis = NewRedis()

func NewRedis() *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr: config.Get("redis.addr"),
	})
	return rdb
}
