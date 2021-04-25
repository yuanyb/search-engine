package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
	"log"
	"math/rand"
	"net/http"
	"search-engine/web/db"
	"strings"
)

// 获取服务器的监控信息
func MonitorHandler(writer http.ResponseWriter, request *http.Request) {
	if !checkLogin(request) {
		writeJson(writer, http.StatusBadRequest, &response{Code: codeFail, Msg: "未登录"})
		return
	}
	_type := strings.TrimSpace(request.FormValue("type"))
	if _type == "" || (_type != "crawler" && _type != "indexer") {
		writeJson(writer, http.StatusBadRequest, &response{
			Code: codeFail,
			Msg:  "参数错误",
		})
		return
	}

	var addrList, deadAddrList []string
	if _type == "crawler" {
		addrList = crawlerAddrList.Load().([]string)
		deadAddrList = deadCrawlerAddrList.Load().([]string)
	} else {
		addrList = indexerAddrList.Load().([]string)
		deadAddrList = deadIndexerAddrList.Load().([]string)
	}
	infoList := requestServerList(addrList, func(channel chan<- interface{}, addr string) {
		resp, err := http.Get(fmt.Sprintf("http://%s/monitor", addr))
		if err != nil {
			log.Println(err)
			channel <- nil
			return
		}

		j, err := simplejson.NewFromReader(resp.Body)
		if err != nil || j.Get("code").MustInt() != codeSuccess {
			log.Println("获取服务器负载失败", err)
			channel <- nil
			return
		}
		channel <- j.Get("data").MustMap()
	})

	var result []map[string]interface{}
	for _, info := range infoList {
		info2 := info.(map[string]interface{})
		result = append(result, info2)
	}
	for _, dead := range deadAddrList {
		result = append(result, map[string]interface{}{
			"addr": dead,
			"dead": true,
		})
	}

	writeJson(writer, http.StatusOK, &response{
		Code: codeSuccess,
		Data: result,
	})
}

func GetCrawlerConfigHandler(writer http.ResponseWriter, request *http.Request) {
	conf, err := db.Mysql.GetCrawlerConfig()
	if err != nil {
		writeJson(writer, http.StatusBadRequest, &response{
			Code: codeFail,
			Msg:  "获取失败",
		})
		return
	}
	writeJson(writer, http.StatusOK, &response{
		Code: codeSuccess,
		Data: conf,
	})
}

// 修改爬虫配置
func UpdateCrawlerConfigHandler(writer http.ResponseWriter, request *http.Request) {
	name := strings.TrimSpace(request.FormValue("name"))
	value := strings.TrimSpace(request.FormValue("value"))
	if name == "" || value == "" {
		writeJson(writer, http.StatusBadRequest, &response{
			Code: codeFail,
			Msg:  "参数错误",
		})
		return
	}

	err := db.Mysql.UpdateCrawlerConfig(name, value)
	if err != nil {
		writeJson(writer, http.StatusBadRequest, &response{
			Code: codeFail,
			Msg:  "修改失败",
		})
		return
	}
	writeJson(writer, http.StatusOK, &response{Code: codeSuccess})
}

// 获取非法关键词
func GetIllegalKeywordHandler(writer http.ResponseWriter, request *http.Request) {
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
	writeJson(writer, http.StatusOK, &response{
		Code: codeSuccess,
		Data: keywords,
	})
}

// 管理非法关键词
func ManageIllegalKeywordHandler(writer http.ResponseWriter, request *http.Request) {
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
	writeJson(writer, http.StatusOK, &response{
		Code: codeSuccess,
	})
}

func GetDomainBlacklistHandler(writer http.ResponseWriter, request *http.Request) {
	blacklist, err := db.Mysql.GetDomainBlacklist()
	if err != nil {
		writeJson(writer, http.StatusInternalServerError, &response{
			Code: codeFail,
			Msg:  "获取失败",
		})
		return
	}
	writeJson(writer, http.StatusOK, &response{
		Code: codeSuccess,
		Data: blacklist,
	})
}

// 管理域名黑名单
func ManageDomainBlacklistHandler(writer http.ResponseWriter, request *http.Request) {
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
	writeJson(writer, http.StatusOK, &response{
		Code: codeSuccess,
	})

}

// 收录域名
func IncludeDomainHandler(writer http.ResponseWriter, request *http.Request) {
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
	if len(addrList) == 0 {
		writeJson(writer, http.StatusOK, &response{Code: codeFail, Msg: "操作失败，无爬虫在运行"})
		return
	}
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
	writeJson(writer, http.StatusOK, &response{Code: codeSuccess})
}

const salt = "QUT-SeArCh"

func AdminLoginHandler(writer http.ResponseWriter, request *http.Request) {
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
	password = hex.EncodeToString(sum[:])
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
	tmpl.Lookup("admin.html").Execute(writer, nil)
}

func writeJson(writer http.ResponseWriter, statusCode int, response *response) {
	j, _ := json.Marshal(response)
	writer.WriteHeader(statusCode)
	_, _ = writer.Write(j)
}

func checkLogin(request *http.Request) bool {
	sessionID, err := request.Cookie("sessionID")
	if err != nil || (sessionID != nil && getSession(sessionID.Value) == nil) {
		return false
	}
	return true
}

func AdminHandler(writer http.ResponseWriter, request *http.Request) {
	// 判断来源 ip，只允许内网访问

	if !checkLogin(request) {
		tmpl.Lookup("admin-login.html").Execute(writer, nil)
		return
	}
	tmpl.Lookup("admin.html").Execute(writer, nil)
}
