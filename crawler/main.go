package main

import (
	"net/http"
	"search-engine/crawler/api"
	"search-engine/crawler/config"
	"search-engine/crawler/core"
	"strconv"
	"strings"
)

func main() {
	goroutineCount, err := strconv.Atoi(config.LocalConfig["crawler.goroutineCount"])
	if err != nil {
		panic("goroutineCount format error")
	}

	engine := core.NewCrawlerEngine(
		core.NewBFScheduler(),
		core.GlobalDl,
		goroutineCount,
		strings.Split(config.LocalConfig["crawler.seedUrls"], ","),
	)
	engine.Run()

	api.Serve(engine)
	_ = http.ListenAndServe("localhost:"+config.LocalConfig["crawler.port"], nil)
}
