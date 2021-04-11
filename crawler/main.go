package main

import (
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

	core.NewCrawlerEngine(
		core.NewBFScheduler(),
		core.GlobalDl,
		goroutineCount,
		strings.Split(config.LocalConfig["crawler.seedUrls"], ","),
	).Run()
}
