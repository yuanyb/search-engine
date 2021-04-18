package db

import (
	"github.com/go-redis/redis/v8"
	"search-engine/crawler/config"
)

var Redis = NewRedis()

func NewRedis() *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr: config.GetLocal("redis.addr"),
	})
	return rdb
}
