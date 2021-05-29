package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
	"log"
	"math"
	"net/http"
	"net/url"
	"search-engine/web/db"
	"search-engine/web/util"
	"sort"
	"strconv"
	"strings"
	"time"
)

var ctx = context.Background()

// 结果项的 url 都是绝对链接，所以为空即可
var baseURL, _ = url.Parse("nil.com")

// 响应代码
const (
	codeSuccess = iota
	codeFail
)

type searchResultItem struct {
	Url          string  `json:"url"`
	Title        string  `json:"title"`
	Abstract     string  `json:"abstract"`
	Score        float64 `json:"score"`
	AnonymousUrl string  `json:"-"`
}

type searchResult struct {
	Query    string
	Items    []*searchResultItem
	Info     string
	Pn       int // 当前页码
	MaxPn    int // 最大页码（<= 10）
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
	// 因为 lrange 命令无法不存在的 key 返回 redis.Nil，所以要判断一下 key 是否存在
	pipeline := db.CacheRedis.Pipeline()
	defer pipeline.Close()
	pExists := pipeline.Exists(ctx, query)
	pItems := pipeline.LRange(ctx, query, int64((pn-1)*10), int64((pn-1)*10+9))
	pItemsTotalLen := pipeline.LLen(ctx, query)
	_, err := pipeline.Exec(ctx)
	if err != nil || pExists.Err() != nil || pItems.Err() != nil || pItemsTotalLen.Err() != nil {
		return nil, err
	} else if pExists.Val() == 0 {
		// 从索引服务器中检索
		return nil, nil
	}

	for _, itemStr := range pItems.Val() {
		item := new(searchResultItem)
		err = json.Unmarshal([]byte(itemStr), item)
		if err != nil {
			continue
		}
		item.AnonymousUrl, _ = convertToProxyURL(baseURL, item.Url)
		result.Items = append(result.Items, item)
	}
	result.Query = query
	result.Pn = pn
	result.MaxPn = int(math.Ceil(float64(pItemsTotalLen.Val()) / 10))
	return result, nil
}

// 将搜索结果添加到缓存
func addToCache(query string, items []*searchResultItem) {
	if len(items) == 0 {
		return
	}
	itemStrList := make([]interface{}, 0, len(items))
	for _, item := range items {
		j, _ := json.Marshal(item)
		itemStrList = append(itemStrList, j)
	}

	pipeline := db.CacheRedis.Pipeline()
	pipeline.RPush(ctx, query, itemStrList...)
	pipeline.Expire(ctx, query, time.Hour*12)
	if _, err := pipeline.Exec(ctx); err != nil {
		log.Println("添加搜索结果到缓存时发生错误", err)
	}
}

// 从索引服务器中检索
func getFromIndexServer(query string) []*searchResultItem {
	addrList := indexerAddrList.Load().([]string)
	resultList := requestServerList(addrList, func(channel chan<- interface{}, addr string) {
		resp, err := http.Get(fmt.Sprintf("http://%s/search?query=%s", addr, url.QueryEscape(query)))
		if err != nil {
			channel <- nil
			log.Println(err)
			return
		}
		defer resp.Body.Close()

		j, err := simplejson.NewFromReader(resp.Body)
		if err != nil {
			channel <- nil
			log.Println(err)
			return
		} else if j.Get("code").MustInt() != codeSuccess {
			log.Println(j.Get("msg").MustString())
			return
		}

		var items []*searchResultItem
		for _, item := range j.Get("data").Get("items").MustArray() {
			it := item.(map[string]interface{})
			score, _ := it["score"].(json.Number).Float64()
			t := &searchResultItem{
				Url:      it["url"].(string),
				Title:    it["title"].(string),
				Abstract: it["abstract"].(string),
				Score:    score,
			}
			items = append(items, t)
		}
		channel <- items
	})

	retItems := make([]*searchResultItem, 0, 100)
	for _, items := range resultList {
		retItems = append(retItems, items.([]*searchResultItem)...)
	}
	return retItems
}

func SearchHandler(writer http.ResponseWriter, request *http.Request) {
	begin := time.Now()
	// 解析参数
	pn := 1
	query := strings.TrimSpace(request.FormValue("query"))
	if strings.TrimSpace(query) == "" {
		servePage(writer, "search-result.gohtml", http.StatusOK, &searchResult{
			Info: "查询内容为空",
		})
		return
	}
	if i, err := strconv.Atoi(request.FormValue("pn")); err == nil {
		pn = i
	}
	if pn >= 11 {
		servePage(writer, "search-result.gohtml", http.StatusOK, &searchResult{Info: "查询内容为空"})
		return
	}

	// query 处理
	if hasIllegalKeywords(query) {
		servePage(writer, "search-result.gohtml", http.StatusOK, &searchResult{
			Info: "搜索内容含有非法关键词",
		})
		return
	}

	// 查 redis 缓存
	if r, err := getFromCache(query, pn); err != nil {
		log.Println("从缓存获取结果时错误", err)
	} else if r != nil {
		r.Duration = time.Now().Sub(begin).Seconds()
		servePage(writer, "search-result.gohtml", http.StatusOK, r)
		return
	}

	// 访问索引服务器检索
	items := getFromIndexServer(query)
	// 按 score 降序排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})
	// 异步添加到缓存
	go addToCache(query, items)

	result := &searchResult{
		Items: items[(pn-1)*10 : util.MinInt((pn-1)*10+10, len(items))],
		Query: query,
		Pn:    pn,
		MaxPn: int(math.Ceil(float64(len(items)) / 10)),
	}
	for _, item := range result.Items {
		item.AnonymousUrl, _ = convertToProxyURL(baseURL, item.Url)
	}
	result.Duration = time.Now().Sub(begin).Seconds()
	servePage(writer, "search-result.gohtml", http.StatusOK, result)
}

func IndexHandler(writer http.ResponseWriter, request *http.Request) {
	if request.RequestURI != "/" {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	tmpl.Lookup("index.html").Execute(writer, nil)
}
