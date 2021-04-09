package main

import (
	"search-engine/index/api"
	"search-engine/index/config"
)

func main() {
	api.Serve(config.GetInt("port"))
}
