// 实时读取配置信息，并作用于爬虫系统，用于后台可以实时手动调节爬虫参数
package config

import (
	"sync/atomic"
	"time"
)

// 爬虫启动参数，做全局变量使用
type CrawlerConfig struct {
	// 随机时间间隔
	RandomInterval bool
	// 时间间隔
	Interval time.Duration
	// 暂停
	Suspend bool
	// 重试次数
	RetryCount int
	// UserAgent
	UserAgent string
	// 日志级别
	LogLevel int
}

var dynamicConfig atomic.Value

func init() {
	go func() {
		for {
			// todo get config from redis
			newConfig := &CrawlerConfig{
				RandomInterval: false,
				Interval:       time.Second,
				Suspend:        false,
				RetryCount:     3,
				UserAgent:      "qut_spider",
				LogLevel:       0,
			}
			dynamicConfig.Store(newConfig)
			time.Sleep(time.Second)
		}
	}()
}

func Get() CrawlerConfig {
	return dynamicConfig.Load().(CrawlerConfig)
}
