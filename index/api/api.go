// 通过 HTTP 对外提供服务
package api

import (
	"bytes"
	"encoding/json"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"io"
	"log"
	"net/http"
	"os"
	"search-engine/index/config"
	"search-engine/index/core"
	"strings"
	"time"
)

var engine *core.Engine

const (
	codeSuccess = iota
	codeFail
)

type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

type MonitorInfo struct {
	Addr                        string  `json:"addr"`
	MemTotal                    int     `json:"mem_total"`
	MemUsed                     int     `json:"mem_used"`
	CpuPercent                  float64 `json:"cpu_percent"`
	RunningTime                 int     `json:"running_time"`
	IndexFileSize               int     `json:"index_size"`
	IndexedDocCount             int     `json:"indexed_doc_count"`
	TokenCount                  int     `json:"token_count"`
	PostingsBufferHitRate       float64 `json:"postings_buffer_hit_rate"`
	TokenDocsCountBufferHitRate float64 `json:"token_docs_count_buffer_hit_rate"`
	DocUrlBufferHitRate         float64 `json:"doc_url_buffer_hit_rate"`
}

func Serve(listenAddr string) {
	engine = core.NewEngine()
	http.HandleFunc("/search", searchHandler)
	// 该接口不应该暴露出去，为了方便忽略了
	http.HandleFunc("/index", indexHandler)
	http.HandleFunc("/monitor", monitor)
	err := http.ListenAndServe(listenAddr, nil)
	log.Fatal(err)
}

func searchHandler(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		write(writer, http.StatusMethodNotAllowed, &Response{Code: codeFail, Msg: "method not allowed"})
		return
	}
	query := request.FormValue("query")
	if strings.TrimSpace(query) == "" {
		write(writer, http.StatusBadRequest, &Response{Code: codeFail, Msg: "param error"})
		return
	}
	start := time.Now()
	searchResults := engine.Search(query)
	searchResults.Duration = time.Now().Sub(start).Milliseconds()
	write(writer, http.StatusOK, &Response{Code: codeSuccess, Data: searchResults})
}

func indexHandler(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPut {
		write(writer, http.StatusMethodNotAllowed, &Response{Code: codeFail, Msg: "method not allowed"})
		return
	}
	data, err := io.ReadAll(request.Body)
	if err != nil {
		log.Println(err.Error())
		write(writer, http.StatusInternalServerError, &Response{Code: codeFail, Msg: "internal server error"})
		return
	}
	var params map[string]string
	if err = json.Unmarshal(data, &params); err != nil {
		log.Println(err.Error())
		write(writer, http.StatusBadRequest, &Response{Code: codeFail, Msg: "json format error"})
		return
	}
	url, document := params["url"], params["document"]
	if url == "" || document == "" {
		write(writer, http.StatusBadRequest, &Response{Code: codeFail, Msg: "param error"})
	}
	engine.AddDocument(url, document)
	write(writer, http.StatusOK, &Response{Code: codeSuccess})
}

func monitor(writer http.ResponseWriter, request *http.Request) {
	info := new(MonitorInfo)
	info.Addr = config.Get("crawler.addr")
	// 获取操作系统信息
	if m, err := mem.VirtualMemory(); err == nil {
		info.MemTotal = int(m.Total)
		info.MemUsed = int(m.Used)
	}
	if c, err := cpu.Percent(time.Millisecond*500, false); err == nil {
		info.CpuPercent = c[0]
	}

	// 获取索引程序信息
	info.RunningTime = int(time.Now().Unix() - engine.Birthday)
	if fileInfo, err := os.Stat(config.Get("boltdb.indexPath")); err == nil {
		info.IndexFileSize = int(fileInfo.Size())
	}
	info.IndexedDocCount = engine.DB.GetDocumentsCount()
	info.TokenCount = engine.DB.GetTokenCount()
	info.DocUrlBufferHitRate = engine.DB.DocUrlBuffer.GetHitRate()
	info.PostingsBufferHitRate = engine.DB.PostingsBuffer.GetHitRate()
	info.TokenDocsCountBufferHitRate = engine.DB.TokenDocsCountBuffer.GetHitRate()

	write(writer, http.StatusOK, &Response{Code: codeSuccess, Data: info})
}

func write(writer http.ResponseWriter, status int, v interface{}) {
	writer.WriteHeader(status)
	buf := &bytes.Buffer{}
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(v)
	b := buf.Bytes()
	n, err := writer.Write(b)
	for retryCount := 0; err != nil && retryCount < 3; retryCount++ {
		b = b[n:]
		n, err = writer.Write(b)
	}
}
