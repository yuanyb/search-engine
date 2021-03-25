package engine

import (
	"math/rand"
	"search-engine/crawler/config"
	"search-engine/crawler/data"
	"search-engine/crawler/downloader"
	"search-engine/crawler/robots"
	"search-engine/crawler/scheduler"
	"search-engine/crawler/util"
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
		go func(no int) {
			util.Logger.Info("爬虫-%d已启动", no)
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
				println("ok:", url)

				// 从网页中提取出 URL、过滤，然后交给调度器
				urls := data.ExtractUrls(url, document)
				e.urlGroupChan <- scheduler.UrlGroup{
					Leader:  url,
					Members: urls,
				}
				e.crawlerWait()
			}
		}(i)
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

		// 手动添加的黑名单

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

// 刚开始过滤 url 是在 crawler goroutines 中进行的，由于涉及并发安全问题，
// 实现起来比较繁琐，所以考虑了一下改在 scheduler goroutine 中进行
func (e *CrawlerEngine) startSchedulerGoroutine() {
	go func() {
		util.Logger.Info("调度模块已启动")
		e.scheduler.AddSeedUrls(e.seedUrls)
		for {
			// urlChan <- url
			for i := 0; i < e.goroutineCount; i++ {
				if e.scheduler.Empty() {
					// 如果调度队列空了，有可能是下面两种情况：
					//   - 刚开始启动，url全交给 crawler 协程了，网页中的链接还没解析，
					//     导致判断时调度队列为空；
					//   - url全部消耗完毕且没有新的url要爬了，但这种情况可能性比较小，
					//     只要种子url足够丰富就不会出现这种情况，可以判断调度队列为空次数，
					//     次数多了就报警
					time.Sleep(time.Millisecond * 500)
					break
				}
				e.urlChan <- e.scheduler.Poll()
			}

			// 等待下载网页
			time.Sleep(time.Millisecond * 100)

			// urlGroup <- urlGroupChan
			urlGroupChanEmpty := false
			for !urlGroupChanEmpty {
				select {
				case urlGroup := <-e.urlGroupChan:
					// 过滤 URL
					urlGroup.Members = e.filterUrl(urlGroup.Members)
					e.scheduler.Offer(urlGroup)
				default:
					urlGroupChanEmpty = true
				}
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
