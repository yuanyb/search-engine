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
		100,
		[]string{"http://www.qut.edu.cn"},
	).Run()
}
