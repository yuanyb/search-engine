// 网页下载器
package downloader

import (
	"compress/gzip"
	"errors"
	"golang.org/x/net/html/charset"
	"io"
	"net/http"
)

// 设置 HTTP 请求的参数
func setHeader(req *http.Request) {
	// 爬虫 UserAgent
	req.Header.Set("User-Agent", "QutSpider")
	// 支持 gzip 压缩传输
	req.Header.Set("Accept-Encoding", "gzip")

}

func decompressGzip(r io.Reader) (io.ReadCloser, error) {
	reader, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	return reader, nil
}

func convertToUtf8(r io.Reader, contentType string) (string, error) {
	reader, err := charset.NewReader(r, contentType)
	if err != nil {
		return "", err
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func decodeDocument(resp *http.Response) (string, error) {
	var err error
	var reader io.ReadCloser
	// 如果返回的是 gzip 压缩过的数据的话
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err = decompressGzip(resp.Body)
	}
	if err != nil {
		return "", err
	}
	defer reader.Close()

	// 字符编码转换
	return convertToUtf8(reader, resp.Header.Get("Content-Type"))
}

func Download(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	setHeader(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", errors.New("")
	}

	return decodeDocument(resp)
}
