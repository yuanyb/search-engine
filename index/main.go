package main

import (
	"context"
	"log"
	"search-engine/index/api"
	"search-engine/index/config"
	"search-engine/index/db"
	"time"
)

// 注册自己到 redis
func registerSelf() {
	go func() {
		addr := config.Get("indexer.listenAddr")
		for {
			// addr:timestamp
			_, err := db.Redis.HSet(context.Background(), "indexer.addr", addr, time.Now().Unix()).Result()
			if err != nil {
				log.Println(addr + "注册到 redis 失败")
			}
			time.Sleep(time.Second * 30) // 每30秒报告自己的存活状态
		}
	}()
}

func main() {
	log.SetFlags(log.LstdFlags | log.Llongfile)
	defer func() {
		// 退出时移除自己
		db.Redis.HDel(context.Background(), "indexer.addr", config.Get("indexer.listenAddr"))
	}()
	registerSelf()
	api.Serve(config.Get("indexer.listenAddr"))
}
