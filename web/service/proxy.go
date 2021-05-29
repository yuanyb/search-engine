// 参考：github.com/asciimoo/morty
package service

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"search-engine/web/util"
	"strconv"
	"strings"
	"time"
)

const (
	maxRedirectCount    = 3
	maxResponseBodySize = 1024 * 1024 * 5

	// 用于在处理 textToken 时能够知道在哪个标签内，然后进行不同的处理
	stateInOther = iota
	stateInStyle
	stateInNoscript
)

var (
	hashKey                 = []byte("QUT_SeArCh")
	httpClient              = http.Client{Timeout: time.Second * 3}
	allowedContentTypeFlags = []string{"html", "css", "image", "plain", "font"}
	charsetPattern          = regexp.MustCompile(`(?i)charset=[\w\d\- ]+;?`)
	cssUrlPattern           = regexp.MustCompile(`(?im)url\(['"]?(.+?)['"]?\)`)

	bodyBanner       = []byte(`<div style="background-color: darkgrey; color:whitesmoke; font-size:medium; border-style: ridge; z-index:999999; position: sticky; top:0; left:0; right:0; text-align: center; padding: 5px;">您正通过隐私代理服务器匿名访问当前网页，您的行为时完全私密的（浏览器和服务器不会记录您的状态信息，但浏览器可能会保存浏览记录）。由于对当前网页禁用了脚本，所以有些网站会显示异常。</div>`)
	formInjection, _ = template.New("form_injection").Parse(`<input type="hidden" name="url" value="{{.url}}" /><input type="hidden" name="key" value="{{.key}}"/>`)

	// https://en.wikipedia.org/wiki/HTML_sanitization
	// https://docs.servicenow.com/bundle/quebec-platform-administration/page/administer/security/concept/c_HTMLSanitizer.html
	// CSS通过 url 追踪隐私：
	//     - https://zhuanlan.zhihu.com/p/33201277
	//     - https://www.cnblogs.com/durcaidurcai/p/11276809.html
	unsafeElements = [][]byte{
		[]byte("applet"),
		[]byte("canvas"),
		[]byte("embed"),
		[]byte("math"),
		[]byte("script"),
		[]byte("svg"),
	}

	safeAttributes = [][]byte{
		[]byte("abbr"), []byte("accesskey"),
		[]byte("align"), []byte("alt"),
		[]byte("as"), []byte("autocomplete"),
		[]byte("charset"), []byte("checked"),
		[]byte("class"), []byte("content"),
		[]byte("contenteditable"), []byte("contextmenu"),
		[]byte("dir"), []byte("for"),
		[]byte("height"), []byte("hidden"),
		[]byte("hreflang"), []byte("id"),
		[]byte("lang"), []byte("media"),
		[]byte("method"), []byte("name"),
		[]byte("nowrap"), []byte("placeholder"),
		[]byte("property"), []byte("rel"),
		[]byte("spellcheck"), []byte("tabindex"),
		[]byte("target"), []byte("title"),
		[]byte("translate"), []byte("type"),
		[]byte("value"), []byte("width"),
	}

	linkRelSafeValues = [][]byte{
		[]byte("alternate"), []byte("archives"),
		[]byte("author"), []byte("copyright"),
		[]byte("first"), []byte("help"),
		[]byte("icon"), []byte("index"),
		[]byte("last"), []byte("license"),
		[]byte("manifest"), []byte("next"),
		[]byte("pingback"), []byte("prev"),
		[]byte("publisher"), []byte("search"),
		[]byte("shortcut icon"), []byte("stylesheet"),
		[]byte("up"),
	}

	linkHttpEquivSafeValues [][]byte = [][]byte{
		// X-UA-Compatible will be added automaticaly, so it can be skipped
		[]byte("date"),
		[]byte("last-modified"),
		[]byte("refresh"), // URL rewrite
		// []byte("location"), TODO URL rewrite
		[]byte("content-language"),
	}
)

func ProxyHandler(writer http.ResponseWriter, request *http.Request) {
	_url := request.FormValue("url")
	key, err := hex.DecodeString(request.FormValue("key"))
	// 验证 hash
	if err != nil || !hmac.Equal(hash(_url), key) {
		serveFailedPage(writer, http.StatusForbidden, "参数验证失败")
		return
	}

	// 复制 get 请求的参数
	request.Form.Del("url")
	request.Form.Del("key")
	params := request.Form.Encode()
	if strings.IndexByte(_url, '?') != -1 {
		_url = fmt.Sprintf("%s&%s", _url, params)
	} else {
		_url = fmt.Sprintf("%s?%s", _url, params)
	}

	process(writer, request, _url, 0)
}

func process(writer http.ResponseWriter, srcReq *http.Request, rawUrl string, redirectCount int) {
	parsedURL, err := url.Parse(rawUrl)
	if err != nil {
		serveFailedPage(writer, http.StatusInternalServerError, "访问目标网页失败")
		return
	}

	var srcBody io.Reader
	if srcReq.Method == "POST" {
		srcBody = srcReq.Body
	}
	req, err := http.NewRequest(srcReq.Method, rawUrl, srcBody)
	if err != nil {
		serveFailedPage(writer, http.StatusInternalServerError, "内部错误")
		return
	}
	copyRequestHeader(req, srcReq)

	resp, err := httpClient.Do(req)
	if err != nil {
		serveFailedPage(writer, http.StatusInternalServerError, "访问目标网页失败")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// 重定向
		switch resp.StatusCode {
		case 301, 302, 303, 307, 308:
			location := resp.Header.Get("Location")
			if location == "" {
				break
			}
			if redirectCount < maxRedirectCount {
				process(writer, srcReq, rawUrl, redirectCount+1)
			} else {
				serveFailedPage(writer, http.StatusInternalServerError, "重定向次数过多")
			}
			return
		}
		serveFailedPage(writer, resp.StatusCode, "访问目标网页失败")
		return
	}

	// 读取响应
	body, err := readBody(writer, resp)
	if err != nil {
		return
	}

	// 向客户端返回 gzip 压缩过的数据
	var gzipWriter io.Writer
	if strings.Contains(srcReq.Header.Get("Accept-Encoding"), "gzip") {
		writer.Header().Set("Content-Encoding", "gzip")
		w := gzip.NewWriter(writer)
		defer w.Close()
		gzipWriter = w
	} else {
		gzipWriter = writer
	}

	// sanitize
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "html") {
		sanitizeHTML(gzipWriter, parsedURL, body)
	} else if strings.Contains(ct, "css") {
		sanitizeCSS(gzipWriter, parsedURL, body)
	} else {
		_, _ = gzipWriter.Write(body)
	}
}

// 将 rawURL 转换成代理地址
func convertToProxyURL(baseURL *url.URL, rawURL string) (string, bool) {
	lowerURL := strings.ToLower(rawURL[:util.MinInt(20, len(rawURL))])
	if strings.HasPrefix(lowerURL, "javascript:") {
		return "", false
	} else if strings.HasPrefix(lowerURL, "data:") {
		l, r := len("data:"), strings.IndexByte(rawURL, ';')
		if r == -1 {
			return "", false
		}
		contentType := rawURL[l:r]
		if strings.Contains(contentType, "image") {
			return rawURL, true
		} else {
			return "", false
		}
	} else if strings.HasPrefix(lowerURL, "#") {
		return rawURL, true
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}

	// 单独取出 fragment，不对fragment哈希
	fragment := ""
	if len(parsedURL.Fragment) > 0 {
		fragment = "#" + parsedURL.Fragment
	}
	parsedURL.Fragment = ""
	parsedURL = baseURL.ResolveReference(parsedURL)

	u := parsedURL.String()
	return fmt.Sprintf("/proxy?url=%s&key=%s%s", url.QueryEscape(u), hex.EncodeToString(hash(u)), fragment), true
}

func sanitizeHTML(writer io.Writer, baseURL *url.URL, body []byte) {
	tokenizer := html.NewTokenizer(bytes.NewReader(body))
	tokenizer.AllowCDATA(true)
	localUnsafeElements := make([][]byte, 0)
	state := stateInOther
	for {
		token := tokenizer.Next()
		if token == html.ErrorToken {
			if tokenizer.Err() != io.EOF {
				_, _ = writer.Write([]byte("目标网页无法解析"))
			}
			break
		}
		if len(localUnsafeElements) == 0 {
			switch token {
			case html.StartTagToken, html.SelfClosingTagToken:
				tagName, hasAttr := tokenizer.TagName()
				if inSlice(tagName, unsafeElements) {
					localUnsafeElements = append(localUnsafeElements, deepCopy(tagName))
					break
				}

				var attrs [][][]byte
				hasMore := true
				for hasAttr && hasMore {
					attrName, attrValue, _hasMore := tokenizer.TagAttr()
					hasMore = _hasMore
					attrs = append(attrs, [][]byte{attrName, attrValue})
				}

				if bytes.Equal(tagName, []byte("base")) {
					for _, attr := range attrs {
						if bytes.Equal(attr[0], []byte("href")) {
							parsedURL, err := url.Parse(string(attr[1]))
							if err == nil {
								baseURL = parsedURL
							}
							break
						}
					}
					break
				} else if bytes.Equal(tagName, []byte("link")) {
					sanitizeLinkTag(writer, baseURL, attrs)
					break
				} else if bytes.Equal(tagName, []byte("meta")) {
					sanitizeMetaTag(writer, baseURL, attrs)
					break
				} else if bytes.Equal(tagName, []byte("noscript")) {
					state = stateInNoscript
					break
				} else if bytes.Equal(tagName, []byte("style")) {
					// noscript 不需要处理标签，直接返回即可，而 style 需要
					state = stateInStyle
				}

				// 写标签和属性
				fmt.Fprintf(writer, "<%s", tagName)
				if hasAttr {
					sanitizeAttrs(writer, baseURL, attrs)
				}
				if token == html.SelfClosingTagToken {
					writer.Write([]byte("/>"))
				} else {
					writer.Write([]byte(">"))
				}

				// 向页面注入额外的内容
				if bytes.Equal(tagName, []byte("form")) {
					// 处理 form 标签，重写 url、添加隐藏参数 key
					var action *url.URL
					for _, attr := range attrs {
						if bytes.Equal(attr[0], []byte("action")) {
							action, _ = url.Parse(string(attr[1]))
							action = baseURL.ResolveReference(action)
							break
						}
					}
					if action == nil {
						action = baseURL
					}
					u := action.String()
					_ = formInjection.Execute(writer, map[string]string{"url": u, "key": hex.EncodeToString(hash(u))})
				} else if bytes.Equal(tagName, []byte("body")) {
					_, _ = writer.Write(bodyBanner)
				}
			case html.EndTagToken:
				t, _ := tokenizer.TagName()
				tagName := string(t)
				if tagName == "style" || tagName == "noscript" {
					state = stateInOther
				}
				// 对于 noscript 标签，由于程序本身会将 js 移除掉，所以应当显示 noscript 的内容，并把 noscript 的标签去掉
				if tagName != "noscript" {
					_, _ = writer.Write(tokenizer.Raw())
				}
			case html.TextToken:
				switch state {
				case stateInOther:
					_, _ = writer.Write(tokenizer.Raw())
				case stateInNoscript:
					sanitizeHTML(writer, baseURL, deepCopy(tokenizer.Raw()))
				case stateInStyle:
					sanitizeCSS(writer, baseURL, tokenizer.Raw())
				}
			case html.DoctypeToken:
				_, _ = writer.Write(tokenizer.Raw())
			}
		} else if token == html.EndTagToken {
			tagName, _ := tokenizer.TagName()
			if bytes.Equal(tagName, localUnsafeElements[len(localUnsafeElements)-1]) {
				localUnsafeElements = localUnsafeElements[:len(localUnsafeElements)-1]
			}
		}
	}
}

func sanitizeCSS(writer io.Writer, baseURL *url.URL, css []byte) {
	urlList := cssUrlPattern.FindAllSubmatchIndex(css, -1)
	if len(urlList) == 0 {
		writer.Write(css)
		return
	}

	pos := 0
	for _, u := range urlList {
		start, end := u[2], u[3]
		if proxyURL, ok := convertToProxyURL(baseURL, string(css[start:end])); ok {
			writer.Write(css[pos:start])
			writer.Write([]byte(proxyURL))
			pos = end
		}
	}
	if pos < len(css) {
		writer.Write(css[pos:])
	}
}

func sanitizeLinkTag(writer io.Writer, baseURL *url.URL, attrs [][][]byte) {
	exclude := false
	for _, attr := range attrs {
		attrName, attrValue := attr[0], attr[1]
		if bytes.Equal(attrName, []byte("rel")) {
			if !inSlice(attrValue, linkRelSafeValues) {
				exclude = true
				break
			}
		} else if bytes.Equal(attrName, []byte("as")) && bytes.Equal(attrValue, []byte("script")) {
			exclude = true
			break
		}
	}
	if !exclude {
		writer.Write([]byte("<link"))
		sanitizeAttrs(writer, baseURL, attrs)
		writer.Write([]byte(">"))
	}
}

func sanitizeMetaTag(writer io.Writer, baseURL *url.URL, attrs [][][]byte) {
	var httpEquiv, content []byte

	for _, attr := range attrs {
		attrName, attrValue := attr[0], attr[1]
		switch string(attrName) {
		case "http-equiv":
			httpEquiv = bytes.ToLower(attrValue)
			if !inSlice(httpEquiv, linkHttpEquivSafeValues) {
				return
			}
		case "content":
			content = attrValue
		case "charset":
			// 响应头的 Content-Type 已经修改为 utf-8 了
			return
		}
	}
	writer.Write([]byte("<meta"))
	urlIndex := bytes.Index(bytes.ToLower(content), []byte("url="))
	if bytes.Equal(httpEquiv, []byte("refresh")) && urlIndex != -1 {
		u := bytes.TrimFunc(content[urlIndex+4:], func(r rune) bool {
			return r == '"' || r == '\''
		})
		if proxyURL, ok := convertToProxyURL(baseURL, string(u)); ok {
			fmt.Fprintf(writer, ` http-equiv="refresh" content="%surl=%s"`, content[:urlIndex], proxyURL)
		}
	} else {
		// 特殊处理，因为 http-equiv 不在 safeAttributes 里
		if len(httpEquiv) > 0 {
			fmt.Fprintf(writer, ` http-equiv="%s"`, httpEquiv)
		}
		sanitizeAttrs(writer, baseURL, attrs)
	}
	writer.Write([]byte(">"))
}

func sanitizeAttrs(writer io.Writer, baseURL *url.URL, attrs [][][]byte) {
	for _, attr := range attrs {
		if inSlice(attr[0], safeAttributes) {
			fmt.Fprintf(writer, ` %s="%s" `, attr[0], html.EscapeString(string(attr[1])))
			continue
		}
		switch string(attr[0]) {
		case "src", "href":
			if proxyURL, ok := convertToProxyURL(baseURL, string(attr[1])); ok {
				fmt.Fprintf(writer, ` %s="%s" `, attr[0], proxyURL)
			}
		case "action":
			// 因为已经向 form 标签注入过 url、key 参数了，所以 action 为 "/proxy" 就行
			writer.Write([]byte(` action="/proxy" `))
		case "style":
			styleValueBuffer := new(bytes.Buffer)
			sanitizeCSS(styleValueBuffer, baseURL, attr[1])
			fmt.Fprintf(writer, ` style="%s" `, html.EscapeString(string(styleValueBuffer.Bytes())))
		}
	}
}

// 根据 hashKey 计算 hmac
func hash(str string) []byte {
	m := hmac.New(sha256.New, hashKey)
	m.Write([]byte(str))
	return m.Sum(nil)
}

func deepCopy(b []byte) []byte {
	t := make([]byte, len(b))
	copy(t, b)
	return t
}

func serveFailedPage(writer http.ResponseWriter, statusCode int, msg string) {
	writer.WriteHeader(statusCode)
	_, _ = writer.Write([]byte(msg))
}

// 判断 ContentType 的资源是否允许访问
func contentTypeAllow(contentType string) bool {
	for _, ct := range allowedContentTypeFlags {
		if strings.Contains(contentType, ct) {
			return true
		}
	}
	return false
}

// 将客户端中的部分请求头参数拷贝到新的代理请求中
func copyRequestHeader(destReq, srcReq *http.Request) {
	// header
	destReq.Header.Set("Accept", srcReq.Header.Get("Accept"))
	destReq.Header.Set("Accept-Language", srcReq.Header.Get("Accept-Language"))
	if strings.Contains(srcReq.Header.Get("Accept-Encoding"), "gzip") {
		// 仅支持 gzip
		destReq.Header.Set("Accept-Encoding", "gzip")
	}
	// 判断请求来源是桌面浏览器还是移动端
	if strings.Contains(srcReq.UserAgent(), "Mobile") {
		destReq.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 7_0_4 like Mac OS X) AppleWebKit/537.51.1 (KHTML, like Gecko) CriOS/31.0.1650.18 Mobile/11B554a Safari/8536.25")
	} else {
		destReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.12; rv:65.0) Gecko/20100101 Firefox/65.0")
	}
}

// 从响应的 Body 中读取字节数据，传入 writer 是为了可以写必要的响应数据（错误、Content-Type）
func readBody(writer http.ResponseWriter, response *http.Response) ([]byte, error) {
	contentType := response.Header.Get("Content-Type")
	if !contentTypeAllow(contentType) {
		serveFailedPage(writer, http.StatusForbidden, "不支持访问该类型的Web资源")
		return nil, errors.New("不支持访问该类型的Web资源")
	}

	var b io.Reader = response.Body

	// 限制 response.body 大小
	if cLen := response.Header.Get("Content-Length"); cLen != "" {
		if l, _ := strconv.Atoi(cLen); l > maxResponseBodySize {
			serveFailedPage(writer, http.StatusForbidden, "Web资源过大，无法访问")
			return nil, errors.New("Web资源过大，无法访问")
		}
	} else {
		b = io.LimitReader(response.Body, maxResponseBodySize)
	}

	// gzip解压缩
	if strings.Contains(response.Header.Get("Content-Encoding"), "gzip") {
		r, err := gzip.NewReader(b)
		if err != nil {
			serveFailedPage(writer, http.StatusForbidden, "访问目标网页失败")
			return nil, err
		} else {
			b = r
			defer r.Close()
		}
	}

	body, err := io.ReadAll(io.LimitReader(b, maxResponseBodySize))
	if err != nil {
		serveFailedPage(writer, http.StatusForbidden, "访问目标网页失败")
		return nil, err
	}

	// 文本解码
	if strings.Contains(contentType, "text") {
		e, eName, _ := charset.DetermineEncoding(body, contentType)
		if !strings.EqualFold(eName, "UTF-8") {
			if body, err = e.NewDecoder().Bytes(body); err != nil {
				serveFailedPage(writer, http.StatusForbidden, "访问目标网页失败")
				return nil, err
			}
		}
	}

	if charsetPattern.MatchString(contentType) {
		contentType = charsetPattern.ReplaceAllString(contentType, "charset=utf-8;")
	} else {
		contentType = contentType + "; charset=utf-8"
	}

	writer.Header().Set("Content-Type", contentType)
	return body, nil
}

func inSlice(v []byte, slice [][]byte) bool {
	for _, v2 := range slice {
		if bytes.Equal(v2, v) {
			return true
		}
	}
	return false
}
