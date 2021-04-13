// 监控模块，实时报告，当前爬虫的信息
package api

import (
	"encoding/json"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"io"
	"net/http"
	"search-engine/crawler/core"
	"sync/atomic"
	"time"
)

type Info struct {
	MemTotal     int     `json:"mem_total"`
	MemUsed      int     `json:"mem_used"`
	CpuPercent   float64 `json:"cpu_percent"`
	RunningTime  int     `json:"running_time"`
	CrawledCount int     `json:"crawled_count"`
	FailureCount int     `json:"failure_count"`
	FailureRate  float32 `json:"failure_rate"`
}

var engine *core.CrawlerEngine

func Serve(e *core.CrawlerEngine) {
	engine = e
	http.HandleFunc("/monitor", monitor)
	http.HandleFunc("/seedurl", addSeedUrl)
}

func monitor(response http.ResponseWriter, request *http.Request) {
	info := new(Info)
	// 获取操作系统信息
	if m, err := mem.VirtualMemory(); err == nil {
		info.MemTotal = int(m.Total)
		info.MemUsed = int(m.Used)
	}
	if c, err := cpu.Percent(time.Millisecond*200, false); err == nil {
		info.CpuPercent = c[0]
	}

	// 获取爬虫信息
	info.CrawledCount = int(atomic.LoadInt32(&engine.CrawledCount))
	info.FailureCount = int(atomic.LoadInt32(&engine.FailureCount))
	if info.CrawledCount != 0 {
		info.FailureRate = float32(info.FailureCount) / float32(info.CrawledCount)
	}
	info.RunningTime = int(time.Now().Unix() - engine.Birthday)

	j, _ := json.Marshal(info)
	for n, err := response.Write(j); err != nil; {
		j = j[n:]
		n, err = response.Write(j)
	}
}

func addSeedUrl(response http.ResponseWriter, request *http.Request) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return
	}
	m := make(map[string]interface{}, 1)
	if err = json.Unmarshal(body, &m); err != nil {
		return
	} else if v, ok := m["seed_urls"]; !ok {
		return
	} else if _, ok = v.([]string); !ok {
		return
	}
	seedUrls := m["seed_urls"].([]string)
	go func() {
		for _, u := range seedUrls {
			engine.SeedUrlChan <- u
		}
	}()
}
