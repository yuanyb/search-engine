package config

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// 本地配置项必须提供
var localConfigItem = [...]string{"mysql.username", "mysql.password", "mysql.host",
	"mysql.port", "mysql.dbname", "sqlite.indexPath", "sqlite.docPath", "indexer.port",
	"redis.addr", "indexer.docUrlBufferSize", "indexer.tokenIdBufferSize", "indexer.postingsBufferSize",
	"indexer.indexerWorkerCount", "indexer.indexerChannelLength", "indexer.postingsBufferFlushThreshold"}

var config map[string]string

func init() {
	path := "./indexer.properties"
	file, err := os.Open(path)
	if err != nil {
		panic(fmt.Sprintf("缺少配置文件 %s", path))
	}

	b, err := io.ReadAll(file)
	if err != nil {
		panic(fmt.Sprintf("读取配置文件 %s 失败", path))
	}

	config = make(map[string]string)
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
	checkLocalConfig(config)
}

func checkLocalConfig(config map[string]string) {
	for _, name := range localConfigItem {
		if _, ok := config[name]; !ok {
			panic(fmt.Sprintf("缺少配置项[%s]", name))
		}
	}
}

func Get(name string) string {
	return config[name]
}

func GetInt(name string) int {
	i, err := strconv.Atoi(config[name])
	if err != nil {
		panic("配置项错误：" + name)
	}
	return i
}
