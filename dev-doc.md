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

### 1.1 config 
遇到了包的环状导入，解决办法：将一方的主动获取变成被动设置

一个有严重问题的写法，如：
```go
select {
    case xxxChan <- queue.Poll(): 
        // 如果向 channel 传递元素失败，每次都会丢失一个元素
    default:
        xxx
}
```
### 1.2 scheduler
### 1.3 robots
### 1.4 engine
### 1.5 monitor
### 1.6 data