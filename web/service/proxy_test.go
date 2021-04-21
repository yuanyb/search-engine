package service

import (
	"bytes"
	"encoding/hex"
	"net/http"
	"net/url"
	"testing"
)

func Test_convertToProxyURL(t *testing.T) {
	type args struct {
		baseURL *url.URL
		rawURL  string
	}
	baseURL, _ := url.Parse("http://base.com/a/b?p=x")
	u3 := "http://test.com/a/b?id=1"
	u4 := "http://base.com/c?id=2"
	wantList := []string{
		3: "./?url_hash=" + hex.EncodeToString(hash(u3)) + "&url=" + url.QueryEscape(u3),
		4: "./?url_hash=" + hex.EncodeToString(hash(u4)) + "&url=" + url.QueryEscape(u4) + "#frag",
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 bool
	}{
		{name: "JS", args: args{baseURL: baseURL, rawURL: "jAvascript:alert('xss')"}, want: "", want1: false},
		{name: "data_html", args: args{baseURL: baseURL, rawURL: "data:text/html;xx"}, want: "", want1: false},
		{name: "data_image", args: args{baseURL: baseURL, rawURL: "data:image/jpeg;xx"}, want: "data:image/jpeg;xx", want1: true},
		{name: "test.com", args: args{baseURL: baseURL, rawURL: "http://test.com/a/b?id=1"}, want: wantList[3], want1: true},
		{name: "/c?id=2", args: args{baseURL: baseURL, rawURL: "/c?id=2#frag"}, want: wantList[4], want1: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := convertToProxyURL(tt.args.baseURL, tt.args.rawURL)
			if got != tt.want {
				t.Errorf("convertToProxyURL() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("convertToProxyURL() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_sanitizeAttrs(t *testing.T) {
	type args struct {
		baseURL *url.URL
		attrs   [][][]byte
	}
	baseURL, _ := url.Parse("http://base.com/a/b")
	proxyURL, _ := convertToProxyURL(baseURL, `http://test.com`)
	tests := []struct {
		name       string
		args       args
		wantWriter string
	}{
		{name: "", args: args{
			baseURL: baseURL,
			attrs: [][][]byte{
				{[]byte("src"), []byte("http://test.com")},
				{[]byte("id"), []byte("id1")},
				{[]byte("onclick"), []byte("alert(123)")},
			},
		}, wantWriter: ` src="` + proxyURL + `"  id="id1" `},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := &bytes.Buffer{}
			sanitizeAttrs(writer, tt.args.baseURL, tt.args.attrs)
			if gotWriter := writer.String(); gotWriter != tt.wantWriter {
				t.Errorf("sanitizeAttrs() = %v, want %v", gotWriter, tt.wantWriter)
			}
		})
	}
}

func Test_sanitizeCSS(t *testing.T) {
	type args struct {
		baseURL *url.URL
		css     []byte
	}
	baseURL, _ := url.Parse("http://base.com/a/b")
	u := "/static/img/a.png"
	proxyURL, _ := convertToProxyURL(baseURL, u)
	tests := []struct {
		name       string
		args       args
		wantWriter string
	}{
		{name: "", args: args{baseURL: baseURL, css: []byte("xxxurl('" + u + "')xxxurl('" + u + "')xxx")}, wantWriter: "xxxurl('" + proxyURL + "')xxxurl('" + proxyURL + "')xxx"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := &bytes.Buffer{}
			sanitizeCSS(writer, tt.args.baseURL, tt.args.css)
			if gotWriter := writer.String(); gotWriter != tt.wantWriter {
				t.Errorf("sanitizeCSS() = %v, want %v", gotWriter, tt.wantWriter)
			}
		})
	}
}

func Test_sanitizeLinkTag(t *testing.T) {
	type args struct {
		baseURL *url.URL
		attrs   [][][]byte
	}
	baseURL, _ := url.Parse("http://base.com/a/b")
	u := "/static/css/main.css"
	proxyURL, _ := convertToProxyURL(baseURL, u)
	tests := []struct {
		name       string
		args       args
		wantWriter string
	}{
		{name: "", args: args{baseURL: baseURL, attrs: [][][]byte{{[]byte("rel"), []byte("not_safe")}}}, wantWriter: ""},
		{name: "", args: args{baseURL: baseURL, attrs: [][][]byte{{[]byte("as"), []byte("script")}}}, wantWriter: ""},
		{name: "", args: args{baseURL: baseURL, attrs: [][][]byte{
			{[]byte("rel"), []byte("stylesheet")},
			{[]byte("src"), []byte(u)}},
		}, wantWriter: `<link rel="stylesheet"  src="` + proxyURL + `" >`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := &bytes.Buffer{}
			sanitizeLinkTag(writer, tt.args.baseURL, tt.args.attrs)
			if gotWriter := writer.String(); gotWriter != tt.wantWriter {
				t.Errorf("sanitizeLinkTag() = %v, want %v", gotWriter, tt.wantWriter)
			}
		})
	}
}

func Test_sanitizeMetaTag(t *testing.T) {
	type args struct {
		baseURL *url.URL
		attrs   [][][]byte
	}
	baseURL, _ := url.Parse("http://base.com/a/b")
	u := "http://base.com/next_page"
	proxyURL, _ := convertToProxyURL(baseURL, u)
	u = url.QueryEscape(u)
	tests := []struct {
		name       string
		args       args
		wantWriter string
	}{
		{
			name: "refresh",
			args: args{baseURL: baseURL, attrs: [][][]byte{
				{[]byte("http-equiv"), []byte("refresh")},
				{[]byte("content"), []byte("xx; url='/next_page'")},
			}},
			wantWriter: `<meta http-equiv="refresh" content="xx; url=` + proxyURL + `">`,
		},
		{
			name: "charset",
			args: args{baseURL: baseURL, attrs: [][][]byte{
				{[]byte("charset"), []byte("utf-8")},
			}},
			wantWriter: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := &bytes.Buffer{}
			sanitizeMetaTag(writer, tt.args.baseURL, tt.args.attrs)
			if gotWriter := writer.String(); gotWriter != tt.wantWriter {
				t.Errorf("sanitizeMetaTag() = %v, want %v", gotWriter, tt.wantWriter)
			}
		})
	}
}

func TestProxyHandler(t *testing.T) {
	baseURL, _ := url.Parse("http://qutsearch.com")
	u, _ := convertToProxyURL(baseURL, "https://baidu.com")
	println(u)
	http.HandleFunc("/", ProxyHandler)
	http.ListenAndServe("localhost:80", nil)
}
