// 网页下载器
package downloader

import (
	"compress/gzip"
	"errors"
	"golang.org/x/net/html/charset"
	"io"
	"net/http"
	"src/config"
	"time"
)

type Downloader struct {
	UserAgent string
	// 一些配置信息...
}

var GlobalDownloader = Downloader{}

// 设置 HTTP 请求的参数
func setHeader(req *http.Request) {
	// 爬虫 Useragent
	req.Header.Set("User-Agent", config.Get().Useragent)
	// 支持 gzip 压缩传输
	req.Header.Set("Accept-Encoding", "gzip")

}

// 解压 gzip 响应
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

func download(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	setHeader(req)

	// 请求失败重试
	retryCount := config.Get().RetryCount
	timeout := config.Get().Timeout
	http.DefaultClient.Timeout = time.Duration(timeout)
	var resp *http.Response
	for i := 0; i < retryCount+1; i++ {
		resp, err = http.DefaultClient.Do(req)
		if err == nil {
			break
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New("")
	}

	return resp, err
}

// 下载网页原始文本
func (d *Downloader) DownloadText(url string) (string, error) {
	resp, err := download(url)
	if err != nil {
		return "", err
	}

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

// 下载二进制文件
func (d *Downloader) DownloadBinary(url string) ([]byte, error) {
	resp, err := download(url)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}
