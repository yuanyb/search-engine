package engine

import (
	"math/rand"
	"src/config"
	"src/data"
	"src/downloader"
	"src/robots"
	"src/scheduler"
	"time"
)

// 网页文档的下载、处理、存储由多个 goroutine 执行
type CrawlerEngine struct {
	// 下载完某个网页文档后解析到的 urlGroup，engine 将其交给 URL 调度器
	urlGroupChan chan scheduler.UrlGroup
	// 传给下载器的 URL
	urlChan chan string
	// 调度策略
	scheduler scheduler.Scheduler
	// 协程数量
	goroutineCount int
	// 种子 URL
	seedUrls []string
}

func (e *CrawlerEngine) startCrawlerGoroutine() {
	for i := 0; i < e.goroutineCount; i++ {
		go func() {
			for {
				// 暂停执行，如果需要的话
				if config.Get().Suspend {
					time.Sleep(time.Second)
					continue
				}

				// 获取下一个 URL 并下载
				url := <-e.urlChan
				document, err := downloader.GlobalDownloader.DownloadText(url)
				if err != nil {
					continue
				}

				// todo document 持久化

				// 从网页中提取出 URL、过滤，然后交给调度器
				urls := data.ExtractUrls(document)
				urls = e.filterUrl(urls)
				e.urlGroupChan <- scheduler.UrlGroup{
					Leader:  url,
					Members: urls,
				}

				e.crawlerWait()
			}
		}()
	}
}

var bloomFilter = NewBloomFilter(1000000)

// 过滤 URL，如：robots.txt禁止爬的，手动添加的不爬的URL，已经爬过的 URL
func (e *CrawlerEngine) filterUrl(urls []string) []string {
	var filterResult []string

	for _, url := range urls {
		// robots
		if !robots.Allow(url, config.Get().Useragent) {
			continue
		}

		// bloomFilter
		if bloomFilter.Has(url) {
			continue
		}
		bloomFilter.Add(url)

		// ...

		// 允许爬取
		filterResult = append(filterResult, url)
	}
	return filterResult
}

func (e *CrawlerEngine) crawlerWait() {
	conf := config.Get()
	if conf.RandomInterval {
		time.Sleep(time.Millisecond * time.Duration(rand.Int63n(conf.Interval)))
	} else {
		time.Sleep(time.Millisecond * time.Duration(conf.Interval))
	}
}

// 刚开始过滤 url 是在 crawler goroutine 中进行的，由于涉及并发安全问题，
// 实现起来比较繁琐，所以考虑了一下改在 scheduler goroutine 中进行
func (e *CrawlerEngine) startSchedulerGoroutine() {
	go func() {
		e.scheduler.AddSeedUrls(e.seedUrls)
		for {
			// urlChan <- url
			full := false
			for !full {
				select {
				case e.urlChan <- e.scheduler.Poll():
				default:
					full = true // urlChan 满了
				}
			}
			// urlGroup <- urlGroupChan
			// 当从 urlGroupChan 中消耗了 goroutineCount 个元素后，urlChan 就空了
			for i := 0; i < e.goroutineCount; i++ {
				urlGroup := <-e.urlGroupChan
				// 过滤 URL
				urlGroup.Members = e.filterUrl(urlGroup.Members)
				e.scheduler.Offer(urlGroup)
			}
		}
	}()
}

// 运行爬虫
func (e *CrawlerEngine) Run() {
	e.startSchedulerGoroutine()
	e.startCrawlerGoroutine()
	<-make(chan byte)
}

func NewCrawlerEngine(sch scheduler.Scheduler, goCount int, seedUrls []string) *CrawlerEngine {
	engine := &CrawlerEngine{
		scheduler:      sch,
		goroutineCount: goCount,
		seedUrls:       seedUrls,
		urlChan:        make(chan string, goCount),
		urlGroupChan:   make(chan scheduler.UrlGroup, goCount),
	}
	return engine
}
