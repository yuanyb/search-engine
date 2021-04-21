package db

import (
	"github.com/go-redis/redis/v8"
	"search-engine/web/config"
)

var CenterRedis = NewRedis()
var CacheRedis = CenterRedis

func NewRedis() *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr: config.Get("redis.addr"),
	})
	return rdb
}
