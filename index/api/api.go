// 通过 HTTP 对外提供服务
package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"search-engine/index/core"
	"strconv"
	"strings"
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

func Serve(port int) {
	engine = core.NewEngine()
	http.HandleFunc("/search", searchHandler)
	// 该接口不因该暴露出去
	http.HandleFunc("/index", indexHandler)
	err := http.ListenAndServe(":"+strconv.Itoa(port), nil)
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
	searchResults := engine.Search(query)
	write(writer, http.StatusOK, searchResults)
}

func indexHandler(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPut {
		write(writer, http.StatusMethodNotAllowed, &Response{Code: codeFail, Msg: "method not allowed"})
		return
	}
	data, err := io.ReadAll(request.Body)
	if err != nil {
		write(writer, http.StatusInternalServerError, &Response{Code: codeFail, Msg: "internal server error"})
		return
	}
	var params map[string]string
	if err = json.Unmarshal(data, &params); err != nil {
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
