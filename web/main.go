package main

import (
	"net/http"
	"search-engine/web/config"
	"search-engine/web/service"
)

func main() {
	http.HandleFunc("/", service.IndexHandler)
	http.HandleFunc("/search", service.SearchHandler)
	http.HandleFunc("/proxy", service.ProxyHandler)
	// admin
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./template/static/")))) // admin.js
	http.HandleFunc("/admin", service.AdminHandler)
	http.HandleFunc("/admin/login", service.AdminLoginHandler)
	http.HandleFunc("/admin/monitor", service.MonitorHandler)
	http.HandleFunc("/admin/include_domain", service.IncludeDomainHandler)
	http.HandleFunc("/admin/manage_illegal_keyword", service.ManageIllegalKeywordHandler)
	http.HandleFunc("/admin/manage_domain_blacklist", service.ManageDomainBlacklistHandler)
	http.HandleFunc("/admin/get_illegal_keyword", service.GetIllegalKeywordHandler)
	http.HandleFunc("/admin/get_domain_blacklist", service.GetDomainBlacklistHandler)
	http.HandleFunc("/admin/get_crawler_config", service.GetCrawlerConfigHandler)
	http.HandleFunc("/admin/update_crawler_config", service.UpdateCrawlerConfigHandler)
	//_ = http.ListenAndServe(config.Get("web.listenAddr"), nil)
	http.ListenAndServeTLS(config.Get("web.ListenAddr"), "./cert.pem", "./private.pem", nil)
}
