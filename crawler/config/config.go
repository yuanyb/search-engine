// 全局配置，实时读取配置信息，并作用于爬虫系统，用于后台可以实时手动调节爬虫参数
package config

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"io"
	"os"
	"search-engine/crawler/util"
	"strings"
	"sync/atomic"
	"time"
)

var (
	dynamicConfig atomic.Value
	db            *sql.DB
	stmt          *sql.Stmt

	// 本地配置项必须提供
	localConfigItem = [...]string{"mysql.username", "mysql.password", "mysql.host",
		"mysql.port", "mysql.dbname", "redis.server", "indexer.server", "crawler.goroutineCount",
		"crawler.seedUrls", "crawler.port"}

	LocalConfig   = loadLocalConfig()
	defaultConfig = CrawlerConfig{
		RandomInterval: false,
		Interval:       3000,
		Suspend:        true,
		Timeout:        10000,
		RetryCount:     3,
		Useragent:      "qut_spider",
		LogLevel:       util.LDebug,
	}
)

// 爬虫启动参数，做全局变量使用
type CrawlerConfig struct {
	// 随机时间间隔
	RandomInterval bool
	// 时间间隔（ms）
	Interval int64
	// 超时时间（ms）
	Timeout int64
	// 暂停
	Suspend bool
	// 重试次数
	RetryCount int
	// Useragent
	Useragent string
	// 日志级别
	LogLevel int
}

func (c *CrawlerConfig) fill(name, value string) {
	switch name {
	case "random_interval": // bool
		util.ToBool(&c.RandomInterval, value)
	case "interval": // int64
		util.ToInt64(&c.Interval, value)
	case "timeout":
		util.ToInt64(&c.Timeout, value)
	case "suspend": // bool
		util.ToBool(&c.Suspend, value)
	case "retry_count": // int
		util.ToInt(&c.RetryCount, value)
	case "useragent": // string
		c.Useragent = value
	case "log_level": // string
		util.ToInt(&c.LogLevel, value)
	}
}

func init() {
	// 初始化数据库
	var err error
	lc := LocalConfig
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8", lc["mysql.username"],
		lc["mysql.password"], lc["mysql.host"], lc["mysql.port"], lc["mysql.dbname"])

	if db, err = sql.Open("mysql", dsn); err != nil {
		panic(err)
	}
	if err = db.Ping(); err != nil {
		panic(err)
	}
	if stmt, err = db.Prepare("select `name`, `value` from `search_engine_crawler`"); err != nil {
		panic(err)
	}

	// 初始化配置更新协程
	initDone := make(chan struct{})
	go func() {
		initialized := false
		for {
			latestConfig := loadLatestConfig()
			// 更新日志级别
			util.Logger.SetLevel(latestConfig.LogLevel)
			dynamicConfig.Store(latestConfig)
			if !initialized {
				initialized = true
				initDone <- struct{}{}
			}
			time.Sleep(time.Second)
		}
	}()
	<-initDone
	close(initDone)
}

func loadLocalConfig() map[string]string {
	file, err := os.Open("./crawler.properties")
	if err != nil {
		pwd, _ := os.Getwd()
		panic(fmt.Sprintf("缺少配置文件 %s/crawler.properties", pwd))
	}

	b, err := io.ReadAll(file)
	if err != nil {
		pwd, _ := os.Getwd()
		panic(fmt.Sprintf("读取配置文件 %s/crawler.properties 失败", pwd))
	}

	config := make(map[string]string)
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue // 忽略注释、空行
		}
		item := strings.SplitN(line, "=", 2)
		if len(item) != 2 {
			panic(fmt.Sprintf("配置项[%s]格式错误", line))
		}
		config[strings.TrimSpace(item[0])] = strings.TrimSpace(item[1])
	}
	checkLocalConfigItem(config)
	return config
}

func checkLocalConfigItem(config map[string]string) {
	for _, name := range localConfigItem {
		if _, ok := config[name]; !ok {
			panic(fmt.Sprintf("缺少配置项[%s]", name))
		}
	}
}

func loadLatestConfig() *CrawlerConfig {
	// 拷贝一份默认配置
	latestConfig := defaultConfig

	rows, err := stmt.Query()
	if err != nil {
		return &latestConfig
	}

	for rows.Next() {
		var name, value string
		if err = rows.Scan(&name, &value); err != nil {
			return &latestConfig
		}
		latestConfig.fill(name, value)
	}
	return &latestConfig
}

func Get() *CrawlerConfig {
	return dynamicConfig.Load().(*CrawlerConfig)
}
