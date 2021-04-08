package main

import (
	"flag"
	"search-engine/index/api"
)

func main() {
	port := flag.Uint("p", 8888, "-p port")
	flag.Parsed()
	api.Serve(*port)
}
