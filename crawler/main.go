package main

import (
	"search-engine/crawler/downloader"
	"search-engine/crawler/engine"
	"search-engine/crawler/scheduler"
)

func main() {
	engine.NewCrawlerEngine(
		scheduler.NewBFScheduler(),
		downloader.GlobalDl,
		10,
		[]string{"https://sina.com.cn"},
	).Run()
}
