package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
	"github.com/go-redis/redis/v8"
	"log"
	"net/http"
	"search-engine/web/db"
	"search-engine/web/util"
	"sort"
	"strconv"
	"strings"
	"time"
)

var ctx = context.Background()

// 响应代码
const (
	codeSuccess = iota
	codeFail
)

type searchResultItem struct {
	Url      string  `json:"url"`
	Title    string  `json:"title"`
	Abstract string  `json:"abstract"`
	Score    float64 `json:"score"`
}

type searchResult struct {
	Query    string
	Items    []*searchResultItem
	Info     string
	Duration float64
}

func hasIllegalKeywords(query string) bool {
	// 拷贝一份切片变量，这样并发修改就不会有问题（忽略可见性）
	illegal := illegalKeywords
	for _, keyword := range illegal {
		if strings.Contains(query, keyword) {
			return true
		}
	}
	return false
}

// 从缓存中获取搜索结果
func getFromCache(query string, pn int) (*searchResult, error) {
	result := new(searchResult)
	r, err := db.Redis.LRange(ctx, query, int64((pn-1)*10), int64((pn-1)*10+10)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	for _, itemStr := range r {
		item := new(searchResultItem)
		err = json.Unmarshal([]byte(itemStr), item)
		if err != nil {
			continue
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

// 将搜索结果添加到缓存
func addToCache(query string, items []*searchResultItem) {
	itemStrList := make([]interface{}, len(items))
	for _, item := range items {
		j, _ := json.Marshal(item)
		itemStrList = append(itemStrList, j)
	}

	pipeline := db.Redis.Pipeline()
	pipeline.RPush(ctx, query, itemStrList...)
	pipeline.Expire(ctx, query, time.Hour*12)
	if _, err := pipeline.Exec(ctx); err != nil {
		log.Println("添加搜索结果到缓存时发生错误", err)
	}
}

// 从索引服务器中检索
func getFromIndexServer(query string) *searchResult {
	addrList := indexerAddrList.Load().([]string)
	resultList := requestServerList(addrList, func(channel chan<- interface{}, addr string) {
		resp, err := http.Get(fmt.Sprintf("%s/search?query=%s", addr, query))
		if err != nil {
			log.Println(err)
			return
		}

		j, err := simplejson.NewFromReader(resp.Body)
		if err != nil {
			log.Println(err)
			return
		} else if j.Get("code").MustInt() != codeSuccess {
			log.Println(j.Get("msg").MustString())
			return
		}

		var items []*searchResultItem
		for _, item := range j.Get("data").Get("Items").MustArray() {
			t := item.(searchResultItem)
			items = append(items, &t)
		}
		channel <- items
	})

	searchResult := new(searchResult)
	for _, items := range resultList {
		searchResult.Items = append(searchResult.Items, items.([]*searchResultItem)...)
	}
	return searchResult
}

func Search(writer http.ResponseWriter, request *http.Request) {
	begin := time.Now()
	// 解析参数
	pn := 1
	query := strings.TrimSpace(request.FormValue("query"))
	if strings.TrimSpace(query) == "" {
		servePage(writer, "search-result.html", http.StatusOK, &searchResult{
			Info: "查询内容为空",
		})
		return
	}
	if i, err := strconv.Atoi(request.FormValue("pn")); err == nil {
		pn = i
	}

	// query 处理
	if hasIllegalKeywords(query) {
		servePage(writer, "search-result.html", http.StatusOK, &searchResult{
			Info: "搜索内容含有非法关键词",
		})
		return
	}

	// 查 redis 缓存
	if r, err := getFromCache(query, pn); err != nil {
		log.Println("从缓存获取结果时错误", err)
	} else if r != nil {
		r.Duration = time.Now().Sub(begin).Seconds()
		servePage(writer, "search-result.html", http.StatusOK, r)
		return
	}

	// 访问索引服务器检索
	result := getFromIndexServer(query)
	sort.Slice(result.Items, func(i, j int) bool { // 按 score 降序排序
		return result.Items[i].Score > result.Items[j].Score
	})
	// 异步添加到缓存
	go addToCache(query, result.Items)

	// 返回
	result.Items = result.Items[(pn-1)*10 : util.MinInt((pn-1)*10+10, len(result.Items))]
	result.Duration = time.Now().Sub(begin).Seconds()
	servePage(writer, "search-result.html", http.StatusOK, result)
}
