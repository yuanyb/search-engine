package core

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/url"
	"search-engine/crawler/config"
	"search-engine/crawler/db"
	"search-engine/crawler/util"
	"strconv"
	"sync/atomic"
	"time"
)

// 网页文档的下载、处理、存储由多个 goroutine 执行
type Engine struct {
	// 下载完某个网页文档后解析到的 urlGroup，engine 将其交给 URL 调度器
	urlGroupChan chan urlGroup
	// 下载器
	downloader Downloader
	// 布隆过滤器
	bloomFilter BloomFilter
	// 传给下载器的 URL，channel 的缓冲区要很长
	urlChan []chan string
	// 调度策略
	scheduler Scheduler
	// 协程数量
	goroutineCount int
	// 种子 URL
	seedUrls    []string
	SeedUrlChan chan string

	// 统计
	Birthday     int64
	CrawledCount int32
	FailureCount int32
}

// urlGroup 表示一个 URL 组，leader 这个 URL 对应页面文档中的所有链接就是 members
type urlGroup struct {
	leader  string
	members []string
}

var indexerAddrList atomic.Value

func InitCron() {
	initDone := make(chan struct{})
	// 索引服务器地址
	go func() {
		initialized := false
		for {
			if r, err := db.Redis.HGetAll(context.Background(), "indexer.addr").Result(); err == nil {
				addrList := make([]string, 0, len(r))
				for addr, heartbeatTime := range r {
					t, _ := strconv.Atoi(heartbeatTime)
					// 40秒内认为存活
					if time.Now().Unix()-int64(t) < 40 {
						addrList = append(addrList, addr)
					}
				}
				indexerAddrList.Store(addrList)
			} else {
				log.Println("获取索引服务器地址失败：" + err.Error())
			}
			if !initialized {
				initialized = true
				initDone <- struct{}{}
			}
			time.Sleep(time.Second * 30)
		}
	}()
	<-initDone
	close(initDone)
}

func (e *Engine) startCrawlerGoroutine() {
	for i := 0; i < e.goroutineCount; i++ {
		go func(num int) {
			defer e.fallback()
			var u string
			for {
				// 暂停执行，如果需要的话
				begin := time.Now()
				if config.Get().Suspend {
					time.Sleep(time.Second)
					continue
				}

				// 获取下一个 URL 并下载，优先从 seedUrlChan 获取
				select {
				case u = <-e.SeedUrlChan:
				default:
					u = <-e.urlChan[num]
				}
				document, err := e.downloader.DownloadText(u)
				if err != nil {
					atomic.AddInt32(&e.FailureCount, 1)
					continue
				}

				// 发送document，从网页中提取出 URL、过滤，然后交给调度器
				atomic.AddInt32(&e.CrawledCount, 1)
				SendDocument(u, document)
				urls := ExtractUrls(u, document)
				urls = e.filterUrl(urls)
				// 打散 url 列表，使各个 crawler goroutine 更加均衡
				util.ShuffleStringSlice(urls)
				e.urlGroupChan <- urlGroup{leader: u, members: urls}
				e.crawlerWait()
				info := fmt.Sprintf("crawler-%d ok, url:%s, time:%.1fs", num, u, time.Now().Sub(begin).Seconds())
				println(info)
			}
		}(i)
	}
}

// 过滤 URL，如：robots.txt禁止爬的，手动添加的不爬的URL，已经爬过的 URL
func (e *Engine) filterUrl(urls []string) []string {
	var filterResult []string

	for _, u := range urls {
		// robots
		if !Allow(u, config.Get().Useragent) {
			continue
		}
		// bloomFilter
		if e.bloomFilter.has(u) {
			continue
		}
		e.bloomFilter.add(u)
		// 允许爬取
		filterResult = append(filterResult, u)
	}
	return filterResult
}

func (e *Engine) crawlerWait() {
	conf := config.Get()
	if conf.RandomInterval {
		time.Sleep(util.Int64ToMillisecond(rand.Int63n(conf.Interval) + 2))
	} else {
		time.Sleep(util.Int64ToMillisecond(conf.Interval))
	}
}

func (e *Engine) fallback() {
	if v := recover(); v != nil {
		panic(v)
		// 报警，备份数据
	}

}

// 改了无数次，终于能按期望效果效果运行了！ ————2021-03-27 0:22
// 之前的爬虫的各个 goroutine 获取 url 都是随意的，
// 如果多个协程同时获得了同一个域名的 url，那么对单个
// 域名来说就会在很短的时间段内有很多请求，爬取间隔的设置
// 也就失去了意义，因此有必要将同一个域名下的 url 绑定到
// 同一个协程中，这样才能解决此问题，使用哈希即可。为了保险
// 起见，不能哈希域名，因为小网站的多个二级域名可能都对应
// 同一个 ip，所以哈希域名对应的ip。
func (e *Engine) startSchedulerGoroutine() {
	go func() {
		defer e.fallback()
		e.scheduler.AddSeedUrls(e.seedUrls)
		for {
			// urlChan <- url
			urlChanFull := false
			for !urlChanFull {
				if e.scheduler.Empty() {
					time.Sleep(util.Int64ToMillisecond(config.Get().Interval + config.Get().Timeout))
					break
				}
				u := e.scheduler.Front()
				to := e.getHostIpHash(u) % e.goroutineCount
				select {
				case e.urlChan[to] <- u:
					e.scheduler.Poll()
				case <-time.After(util.Int64ToMillisecond(config.Get().Interval + config.Get().Timeout)):
					urlChanFull = true
				}
			}

			// urlGroup <- urlGroupChan
			urlGroupChanEmpty := false
			for !urlGroupChanEmpty {
				select {
				case urlGroup := <-e.urlGroupChan:
					e.scheduler.Offer(urlGroup)
				default:
					urlGroupChanEmpty = true
				}
			}
		}
	}()
}

var ipHashMap = make(map[string]int, 10000)

// 获取 host 对应 ip 的 hash，作用是将属于同一个 ip 的 url
// 全交给同一个协程爬取
func (e *Engine) getHostIpHash(rawUrl string) int {
	parsedUrl, err := url.Parse(rawUrl)
	if err != nil {
		return rand.Intn(e.goroutineCount)
	}
	if h, ok := ipHashMap[parsedUrl.Host]; ok {
		return h
	}

	var ip []net.IP
	retryCount := config.Get().RetryCount
	for i := 0; i < retryCount+1; i++ {
		if ip, err = net.LookupIP(parsedUrl.Host); err != nil {
			continue
		}
		break
	}
	// 如果重试后还是无法获取 ip
	if err != nil || len(ip) == 0 {
		return rand.Intn(e.goroutineCount)
	}

	h := util.HashByteSlice(ip[0])
	ipHashMap[parsedUrl.Host] = h
	return h
}

// 运行爬虫
func (e *Engine) Run() {
	e.startSchedulerGoroutine()
	e.startCrawlerGoroutine()
}

func NewCrawlerEngine(sch Scheduler, dl Downloader, bf BloomFilter, goCount int, seedUrls []string) *Engine {
	var chanList = make([]chan string, goCount)
	for i := 0; i < goCount; i++ {
		// 大容量的 buffered channel 是为了能让 crawler goroutine 都能有事干，
		// 如果是 unbuffered channel，由于 url 过于集中，连续很多都是同一个网站的 url，
		// 造成其他网站的 url 得不到爬取，很多协程都处于空闲状态
		chanList[i] = make(chan string, 10000)
	}
	engine := &Engine{
		scheduler:      sch,
		downloader:     dl,
		bloomFilter:    bf,
		goroutineCount: goCount,
		seedUrls:       seedUrls,
		SeedUrlChan:    make(chan string),
		urlChan:        chanList,
		urlGroupChan:   make(chan urlGroup, goCount*100),
		Birthday:       time.Now().Unix(),
	}
	return engine
}
