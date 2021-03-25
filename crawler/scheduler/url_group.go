package scheduler

// UrlGroup 表示一个 URL 组，Leader 这个 URL 对应页面文档中的所有链接就是 Members
type UrlGroup struct {
	Leader  string
	Members []string
}
