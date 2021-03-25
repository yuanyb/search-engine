package main

import (
	"search-engine/crawler/engine"
	"search-engine/crawler/scheduler"
)

func main() {
	engine.NewCrawlerEngine(
		scheduler.NewBFScheduler(),
		1,
		[]string{"https://fuliba2020.net/"},
	).Run()
	// todo  删除 url后的 #xxx
}
