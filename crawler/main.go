package main

import (
	"context"
	"log"
	"net/http"
	"search-engine/crawler/api"
	"search-engine/crawler/config"
	"search-engine/crawler/core"
	"search-engine/crawler/db"
	"strconv"
	"strings"
	"time"
)

// 注册自己到 redis
func registerSelf() {
	go func() {
		addr := config.GetLocal("crawler.listenAddr")
		for {
			// addr:timestamp
			_, err := db.Redis.HSet(context.Background(), "crawler.addr", addr, time.Now().Unix()).Result()
			if err != nil {
				log.Println(addr + "注册到 redis 失败")
			}
			time.Sleep(time.Second * 30) // 每30秒报告自己的存活状态
		}
	}()
}

func main() {
	defer func() {
		// 退出时移除自己
		db.Redis.HDel(context.Background(), "crawler.addr", config.GetLocal("crawler.listenAddr"))
	}()
	goroutineCount, err := strconv.Atoi(config.GetLocal("crawler.goroutineCount"))
	if err != nil {
		panic("goroutineCount format error")
	}

	core.InitCron()
	engine := core.NewCrawlerEngine(
		core.NewBFScheduler(),
		core.GlobalDl,
		goroutineCount,
		strings.Split(config.GetLocal("crawler.seedUrls"), ","),
	)
	engine.Run()

	registerSelf()
	api.Serve(engine)
	_ = http.ListenAndServe(config.GetLocal("crawler.listenAddr"), nil)
}
