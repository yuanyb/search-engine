package db

import (
	"github.com/go-redis/redis/v8"
	"search-engine/web/config"
)

var CenterRedis = NewRedis()
var CacheRedis = CenterRedis // todo 缓存应该使用一个单独的 redis

func NewRedis() *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr: config.Get("redis.addr"),
	})
	return rdb
}
