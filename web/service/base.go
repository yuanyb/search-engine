package service

import (
	"crypto/rand"
	"encoding/hex"
	"html/template"
	"io"
	"log"
	"net/http"
	"search-engine/web/db"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

func init() {
	initCron()
	initTemplate()
}

var (
	illegalKeywords []string
	indexerAddrList atomic.Value
	crawlerAddrList atomic.Value
	tpl             *template.Template
)

// 定时任务协程
//     - 更新非法关键词
//     - 获取最新 indexer 服务器地址
//	   - 获取最新 crawler 服务器地址
func initCron() {
	wg := sync.WaitGroup{}
	wg.Add(2)
	// 定期更新非法关键词
	go func() {
		initialized := false
		for {
			illegal, err := db.Mysql.GetIllegalKeyWords()
			if err == nil {
				// 实时性要求低，不用做并发安全处理
				illegalKeywords = illegal
			}
			if !initialized {
				wg.Done()
				initialized = true
			}
			time.Sleep(time.Minute)
		}
	}()
	// 定期刷新 crawler 和 indexer 服务器地址
	go func() {
		initialized := false
		for {
			// 索引服务器地址
			if r, err := db.Redis.HGetAll(ctx, "indexer.addr").Result(); err != nil {
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
			// 爬虫服务器地址
			if r, err := db.Redis.HGetAll(ctx, "crawler.addr").Result(); err != nil {
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
				log.Println("获取爬虫服务器地址失败：" + err.Error())
			}
			if !initialized {
				initialized = true
				wg.Done()
			}
			time.Sleep(time.Second * 30)
		}
	}()
	wg.Wait()
}

func initTemplate() {
	t, err := template.ParseGlob("./template/*.html")
	if err != nil {
		log.Fatalln(err)
	}
	tpl = t
}

// 请求服务器地址列表中的每个地址，并获得返回结果
func requestServerList(addrList []string, f func(channel chan<- interface{}, addr string)) []interface{} {
	resultChannel := make(chan interface{}, len(addrList))
	// 启动 len(addrList) 个 goroutine 访问检索服务器检索
	for _, addr := range addrList {
		go f(resultChannel, addr)
	}

	// 从 resultChannel 中获取结果
	result := make([]interface{}, len(addrList))
	deadline := time.Now().Add(time.Second * 3).UnixNano() // 截止时间戳
	for time.Now().UnixNano() < deadline {
		select {
		case r := <-resultChannel:
			result = append(result, r)
		case <-time.After(time.Duration(deadline - time.Now().UnixNano())): // 到 deadline 的剩余时间
		}
	}
	return result
}

func servePage(writer http.ResponseWriter, tplName string, statusCode int, data interface{}) {
	writer.WriteHeader(statusCode)
	if err := tpl.Lookup(tplName).Execute(writer, data); err != nil {
		log.Printf("模板[%s]执行时发生错误：%s\n", tplName, err)
	}
}

// 简单的 session，存管理员登陆状态
type session struct {
	sessionId      string
	lastAccessTime int64
	data           map[string]interface{}
}

var sessionMap = sync.Map{}

func init() {
	go func() {
		for {
			sessionMap.Range(func(key, value interface{}) bool {
				if time.Now().Unix()-value.(*session).lastAccessTime > 3600*24 {
					sessionMap.Delete(key)
				}
				return true
			})
			time.Sleep(time.Minute)
		}
	}()
}

func newSession() *session {
	b := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, b)
	if err != nil {
		return nil
	}
	sessionId := hex.EncodeToString(b)
	s := &session{
		sessionId:      sessionId,
		lastAccessTime: time.Now().Unix(),
	}
	sessionMap.Store(sessionId, s)
	return s
}

func getSession(sessionId string) *session {
	s, _ := sessionMap.Load(sessionId)
	s2 := s.(*session)
	s2.lastAccessTime = time.Now().Unix()
	return s2
}

func delSession(sessionId string) {
	sessionMap.Delete(sessionId)
}
