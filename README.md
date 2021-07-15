使用 go 语言实现的 HuginnServer —— 分布式爬虫框架 的客户端，附带了一个爬取考试成绩的简单例子。

Usage:
```
//go.mod
require "repo.mazhangjing.com/go-huginn-task-client" latest
```
```go
package main

import (
	"fmt"
	"repo.mazhangjing.com/go-huginn-task-client/huginn"
)

func main() {
	huginn.HuginnLoginUrl = "http://xxxxxxxxx.com"
	job, err := huginn.FetchJob("CM101", "zk2021", 3, 100)
	if err != nil {
		return
	}
	fmt.Printf("%#v", job)
}
```