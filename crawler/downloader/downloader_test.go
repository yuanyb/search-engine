package downloader

import (
	"fmt"
	"testing"
)

func TestDownload(t *testing.T) {
	text, err := GlobalDownloader.DownloadText("https://baidu.com")
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println(text)
}
