package main

import (
	"net/http"
	"search-engine/web/config"
	"search-engine/web/service"
)

func main() {
	// todo 匿名访问链接有问题
	http.HandleFunc("/", service.Index)
	http.HandleFunc("/search", service.Search)
	http.HandleFunc("/proxy", service.ProxyHandler)
	_ = http.ListenAndServe(config.Get("web.listenAddr"), nil)
}
