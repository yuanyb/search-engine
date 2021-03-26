# 开发文档

## 1. 爬虫系统
> **TODO:**
>   - 网页更新
>   - IP 代理
>   - 监控
>   - 单机模式下，定时保存队列中的 URL 和 BloomFilter序列化后的数据，进行备份，使得爬虫异常终止时可以继续任务
>   - 分布式 URL 调度，暂时考虑使用 Redis
>   - 搜索建议使用百度的接口，一方面是不好实现，另一方面是不追踪隐私，意味着不能记录用户的搜索内容
>       - 接口：http://suggestion.baidu.com/su?wd=[内容]

(1) 遇到了包的环状导入，解决办法：将一方的主动获取变成被动设置

(2) 一个有严重问题的写法，如：
```go
select {
case xxxChan <- queue.Poll(): 
    // 如果向 channel 传递元素失败，每次都会丢失一个元素
default:
    xxx
}
```

(3) 目前的爬虫的各个 goroutine 获取 url 都是随意的，
如果多个协程同时获得了同一个域名的 url，那么对单个
域名来说就会在很短的时间段内有很多请求，爬取间隔的设置
也就失去了意义，因此有必要将同一个域名下的 url 绑定到
同一个协程中，这样才能解决此问题，使用哈希即可。为了保险
起见，不能哈希域名，因为小网站的多个二级域名可能都对应
同一个 ip，所以哈希域名对应的ip。
    刚开始有陷入了问题(2)，`xxxChan <- queue.Poll()` 
会丢失 url，~~~最后想到了个办法，就是使用 buffered channel，
由于网页上的链接大都是指向本站，因此 buffered channel 的
长度要足够大，否则会阻塞很久。~~~
    经测试，buffered channel 也没用，还是会存满，因为 crawler
协程消耗一个 url 却要产生几十数百个 url，因此总会存满，总是存满
的话使用 unbuffered channel 是一样的。
新方案：
```go
select {
case urlChan[to] <- scheduler.Poll():
case time.After(n):
    // 等待 n 时间，如果还不能发送到对应 channel 的话，
    // 就尝试给其他 crawler 协程
}
```
<br>
    新问题，由于 url 过于集中，连续很多都是同一个网站的 url，
造成其他网站的 url 得不到爬取，浪费了很多协程。
想到两个办法：

- 调度器中的队列，随机打乱，但就不是按照网页质量爬了
- buffer channel，因此还是给很大的 buffer，能够减轻一些这种状况，
  然后，将 group.members 随机打散
  
    还是无限阻塞，只能在 Scheduler 接口加个 Front 方法获取队列首元素了，
这样再结合 `len(channel) == cap(channel)` 来判断是否阻塞。
  
(4) 有很多次无限阻塞住了，这种情况可以用 select 测试到底是哪出问题了