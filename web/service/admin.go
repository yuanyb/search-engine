package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
	"log"
	"math/rand"
	"net/http"
	"search-engine/web/db"
	"strings"
)

type response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// 获取服务器的监控信息
func Monitor(writer http.ResponseWriter, request *http.Request) {
	if !checkLogin(request) {
		writeJson(writer, http.StatusBadRequest, &response{Code: codeFail, Msg: "未登录"})
		return
	}
	// todo 是否登录
	result := make(map[string]interface{}, 2)
	f := func(addrList []string, key string) {
		result[key] = requestServerList(addrList, func(channel chan<- interface{}, addr string) {
			resp, err := http.Get(fmt.Sprintf("%s/monitor", addr))
			if err != nil {
				log.Println(err)
				return
			}

			j, err := simplejson.NewFromReader(resp.Body)
			if err != nil {
				log.Println("获取服务器负载失败", err)
				return
			}

			channel <- j
		})
	}
	f(crawlerAddrList.Load().([]string), "crawler_monitor_info_list")
	f(indexerAddrList.Load().([]string), "indexer_monitor_info_list")

	writeJson(writer, http.StatusOK, &response{
		Code: codeSuccess,
		Data: result,
	})
}

// 从 redis 中移除（索引、爬虫）服务器地址
func RemoveServer(writer http.ResponseWriter, request *http.Request) {
	if !checkLogin(request) {
		writeJson(writer, http.StatusBadRequest, &response{Code: codeFail, Msg: "未登录"})
		return
	}
	addr := request.FormValue("addr")
	target := request.FormValue("target")
	if len(addr) == 0 || len(target) == 0 || (target != "crawler" && target != "indexer") {
		writeJson(writer, http.StatusBadRequest, &response{
			Code: codeFail,
			Msg:  "参数错误",
		})
		return
	}

	var j []byte
	if _, err := db.Redis.SRem(context.Background(), target+".addr", addr).Result(); err != nil {
		writeJson(writer, http.StatusInternalServerError, &response{
			Code: codeFail,
			Msg:  "移除失败",
		})
		return
	}
	j, _ = json.Marshal(&response{
		Code: codeSuccess,
	})
	_, _ = writer.Write(j)
}

// 获取非法关键词
func GetIllegalKeyword(writer http.ResponseWriter, request *http.Request) {
	if !checkLogin(request) {
		writeJson(writer, http.StatusBadRequest, &response{Code: codeFail, Msg: "未登录"})
		return
	}
	keywords, err := db.Mysql.GetIllegalKeyWords()
	if err != nil {
		writeJson(writer, http.StatusInternalServerError, &response{
			Code: codeFail,
			Msg:  "获取失败",
		})
	}
	writeJson(writer, http.StatusInternalServerError, &response{
		Code: codeSuccess,
		Data: keywords,
	})
}

// 管理非法关键词
func ManageIllegalKeyword(writer http.ResponseWriter, request *http.Request) {
	if !checkLogin(request) {
		writeJson(writer, http.StatusBadRequest, &response{Code: codeFail, Msg: "未登录"})
		return
	}
	// 处理参数
	keyword := strings.TrimSpace(request.FormValue("keyword"))
	opType := strings.TrimSpace(request.FormValue("opType"))
	keywordList := strings.Split(keyword, "|") // add的话
	putIdx := 0
	for i := 0; i < len(keywordList); i++ {
		if k := strings.TrimSpace(keywordList[i]); k != "" {
			keywordList[putIdx] = k
			putIdx++
		}
	}
	keywordList = keywordList[:putIdx]
	if keyword == "" || opType == "" || (opType != "add" && opType != "del") {
		writeJson(writer, http.StatusBadRequest, &response{
			Code: codeFail,
			Msg:  "参数错误",
		})
		return
	}

	// 访问数据库
	if opType == "add" {
		if err := db.Mysql.AddIllegalKeywords(keywordList); err != nil {
			writeJson(writer, http.StatusInternalServerError, &response{Code: codeFail, Msg: "操作失败"})
			return
		}
	} else if opType == "del" {
		if err := db.Mysql.DelIllegalKeyword(keyword); err != nil {
			writeJson(writer, http.StatusInternalServerError, &response{Code: codeFail, Msg: "操作失败"})
			return
		}
	}
	writeJson(writer, http.StatusInternalServerError, &response{
		Code: codeSuccess,
	})
}

func GetDomainBlacklist(writer http.ResponseWriter, request *http.Request) {
	blacklist, err := db.Mysql.GetDomainBlacklist()
	if err != nil {
		writeJson(writer, http.StatusInternalServerError, &response{
			Code: codeFail,
			Msg:  "获取失败",
		})
	}
	writeJson(writer, http.StatusInternalServerError, &response{
		Code: codeSuccess,
		Data: blacklist,
	})
}

// 管理域名黑名单
func ManageDomainBlacklist(writer http.ResponseWriter, request *http.Request) {
	if !checkLogin(request) {
		writeJson(writer, http.StatusBadRequest, &response{Code: codeFail, Msg: "未登录"})
		return
	}
	// 处理参数
	domain := strings.TrimSpace(request.FormValue("domain"))
	opType := strings.TrimSpace(request.FormValue("opType"))
	if domain == "" || opType == "" || (opType != "add" && opType != "del") {
		writeJson(writer, http.StatusBadRequest, &response{
			Code: codeFail,
			Msg:  "参数错误",
		})
		return
	}
	domainList := strings.Split(domain, "|") // add的话
	putIdx := 0
	for i := 0; i < len(domainList); i++ {
		if d := strings.TrimSpace(domainList[i]); d != "" {
			domainList[putIdx] = d
			putIdx++
		}
	}
	domainList = domainList[:putIdx]

	// 访问数据库
	if opType == "add" {
		if err := db.Mysql.AddDomainBlacklist(domainList); err != nil {
			writeJson(writer, http.StatusInternalServerError, &response{
				Code: codeFail,
				Msg:  "操作失败",
			})
			return
		}
	} else {
		if err := db.Mysql.DelDomainBlacklist(domain); err != nil {
			writeJson(writer, http.StatusInternalServerError, &response{
				Code: codeFail,
				Msg:  "操作失败",
			})
			return
		}
	}
	writeJson(writer, http.StatusInternalServerError, &response{
		Code: codeSuccess,
	})

}

// 收录域名
func IncludeDomain(writer http.ResponseWriter, request *http.Request) {
	if !checkLogin(request) {
		writeJson(writer, http.StatusBadRequest, &response{Code: codeFail, Msg: "未登录"})
		return
	}
	domain := request.FormValue("domain")
	domainList := strings.Split(domain, "|")
	putIdx := 0
	for _, d := range domainList {
		if d = strings.TrimSpace(d); d != "" {
			domainList[putIdx] = d
			putIdx++
		}
	}
	domainList = domainList[:putIdx]
	if len(domainList) == 0 {
		writeJson(writer, http.StatusBadRequest, &response{Code: codeFail, Msg: "参数错误"})
		return
	}

	addrList := crawlerAddrList.Load().([]string)
	addr := addrList[rand.Intn(len(addrList))]
	b, _ := json.Marshal(map[string]interface{}{
		"seed_urls": domainList,
	})
	resp, err := http.Post(addr+"/seedurl", "application/json", bytes.NewReader(b))
	if err != nil {
		writeJson(writer, http.StatusInternalServerError, &response{Code: codeFail, Msg: "收录失败"})
		return
	}
	j, err := simplejson.NewFromReader(resp.Body)
	if err != nil || j.Get("code").MustInt() != codeSuccess {
		writeJson(writer, http.StatusInternalServerError, &response{Code: codeFail, Msg: "收录失败"})
		return
	}
	writeJson(writer, http.StatusInternalServerError, &response{Code: codeSuccess})
}

const salt = "QUT-SeArCh"

func Login(writer http.ResponseWriter, request *http.Request) {
	username := strings.TrimSpace(request.FormValue("username"))
	password := strings.TrimSpace(request.FormValue("password"))
	if username == "" || password == "" {
		writeJson(writer, http.StatusOK, &response{
			Code: codeFail,
			Msg:  "用户名或密码为空",
		})
		return
	}
	sum := sha256.Sum256([]byte(password + salt))
	password = string(sum[:])
	if ok, err := db.Mysql.Login(username, password); err != nil || !ok {
		writeJson(writer, http.StatusOK, &response{
			Code: codeFail,
			Msg:  "用户名或密码错误",
		})
		return
	}
	sess := newSession()
	http.SetCookie(writer, &http.Cookie{
		Name:  "sessionID",
		Value: sess.sessionId,
	})
	writeJson(writer, http.StatusOK, &response{
		Code: codeSuccess,
		Data: map[string]string{"redirect": "/admin"},
	})
}

func writeJson(writer http.ResponseWriter, statusCode int, response *response) {
	j, _ := json.Marshal(response)
	writer.WriteHeader(statusCode)
	_, _ = writer.Write(j)
}

func checkLogin(request *http.Request) bool {
	_, err := request.Cookie("sessionID")
	if err != nil {
		return false
	}
	return true
}
