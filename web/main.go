package main

import (
	"net/http"
	"search-engine/web/service"
)

func main() {
	http.HandleFunc("/search", service.Search)
}
