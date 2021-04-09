package main

import (
	"search-engine/crawler/core"
)

func main() {
	core.NewCrawlerEngine(
		core.NewBFScheduler(),
		core.GlobalDl,
		100,
		[]string{"http://www.qut.edu.cn"},
	).Run()
}
