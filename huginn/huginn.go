package huginn

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

var HuginnBaseUrl = "http://ct.mazhangjing.com:8099"
var HuginnLoginUrl = "http://ct.mazhangjing.com:8099/login"

var USERNAME = "corkine"
var PASSWORD = "spring123456"

//Task 指的是一条查询的结构
type Task struct {
	Id int `json:"id"`
	RemainFailedRetryTime int `json:"remainFailedRetryTime"`
	WorkerPromiseReturnSeconds int `json:"workerPromiseReturnSeconds"`
	TaskGroup string `json:"taskGroup"`
	Data string `json:"data"`
	Status string `json:"status"`
	Result string `json:"result"`
	Information string `json:"information"`
	UpdateTime string `json:"updateTime"`
}

//FinishedTask 指的是写入 Task 结果后数据库返回的值
type FinishedTask struct {
	WorkStatus string `json:"workStatus"`
	Note string `json:"note"`
	FinishStatus string `json:"finishStatus"`
	GroupId string `json:"groupId"`
	TaskId int `json:"taskId"`
	Task Task `json:"bean"`
}

//printErr 用于打印错误到日志（后期可能提供其他实现）
func printErr(err error) {
	fmt.Printf("ERROR: %#v\n", err)
}

//login 用于登录 Huginn Server，用于后续 API 获取 Cookie 令牌
func login() (cookie http.Cookie, err error) {
	client := http.Client{}
	form, err := client.PostForm(HuginnLoginUrl, map[string][]string{
		"userName": {USERNAME},
		"passWord": {PASSWORD},
	})
	if err != nil {
		printErr(err)
		return http.Cookie{}, err
	}
	all, _ := io.ReadAll(form.Body)
	content := string(all)
	//fmt.Printf("%#v, %s", form, content)
	log.Println("Login successful with " + content)
	if !strings.Contains(content,"USER") &&
		!strings.Contains(content,"ADMIN") {
		return *form.Cookies()[0], fmt.Errorf("login failed %s", content)
	}
	//fmt.Printf("%#v", form.Cookies()[0])
	return *form.Cookies()[0], nil
}

//GetStatus 获取当前组 groupName 的状态，其中 kind 可以为 ALL,NEW,SUSPEND,FINISHED 等数据库 status 状态字段
func GetStatus(groupName, kind string) (data []Task, err error) {
	cookie, _ := login()
	fmt.Printf("%v", cookie)
	groupTaskURL := HuginnBaseUrl + "/" + "/task/" + groupName
	statusURL := groupTaskURL + "/fetch?type=" + kind
	req, _ := http.NewRequest("GET",statusURL,nil)
	req.AddCookie(&cookie)
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		printErr(err)
		return nil, err
	}
	bytes,_ := io.ReadAll(resp.Body)
	var tasks []Task
	err = json.Unmarshal(bytes, &tasks)
	if err != nil {
		printErr(err)
		return nil, err
	}
	return tasks, err
}

//PushTask 为某个组 groupName 写入新的查询任务 newData，其中 []string 包含数据库 data 字段
func PushTask(groupName string, maxRetry int, newData []string) (data string) {
	pushURL := HuginnBaseUrl + "/task/push"
	cookie, err := login()
	if err != nil {
		printErr(err)
		return
	}
	client := http.Client{}
	type TaskUpload struct {
		Group string `json:"group"`
		Data string `json:"data"`
	}
	var appendData []TaskUpload
	for _, dta := range newData {
		appendData = append(appendData, TaskUpload{
			Group: groupName,
			Data: dta,
		})
	}
	marshal, err := json.Marshal(appendData)
	if err != nil {
		printErr(err)
		return
	}
	request, err := http.NewRequest("GET",
		fmt.Sprintf("%s&maxWorkerRetryTime=%d", pushURL, maxRetry),
		strings.NewReader(string(marshal)))
	if err != nil {
		printErr(err)
		return
	}
	request.AddCookie(&cookie)
	do, err := client.Do(request)
	if err != nil {
		printErr(err)
		return
	}
	all, _ := io.ReadAll(do.Body)
	data = string(all)
	return
}

//FetchJob 从某个 groupName 获取一个 Task，作为一个 Job，相比较 Task，Job 添加了 runnerName 执行者信息
//fetchCount 最大尝试次数，returnGroupInSeconds 最晚多长时间返回结果给服务器（如果超时，则任务 status 被看做 NEW，
//且 fetchCount - 1，当 fetchCount 为 0 后，status 被置为 SUSPEND）
func FetchJob(runnerName,groupName string, fetchCount,returnGroupInSeconds int) ([]Task, error) {
	groupJobURL := HuginnBaseUrl + "/job/" + groupName
	getURL := groupJobURL + "/fetch"
	cookie, err := login()
	if err != nil {
		printErr(err)
		return nil, err
	}
	client := http.Client{}
	request, err := http.NewRequest("GET",
		fmt.Sprintf("%s?groupId=%s&number=%d&runner=%s&workerPromiseReturnSeconds=%d",
			getURL,groupName,fetchCount,runnerName,returnGroupInSeconds), nil)
	if err != nil {
		printErr(err)
		return nil, err
	}
	request.AddCookie(&cookie)
	do, err := client.Do(request)
	if err != nil {
		printErr(err)
		return nil, err
	}
	all, _ := io.ReadAll(do.Body)
	var result []Task
	err = json.Unmarshal(all, &result)
	if err != nil {
		log.Printf("%#v", string(all))
		printErr(err)
		return nil, err
	}
	return result, err
}

//FinishJob 将一个已经获取到结果的 Job 信息写回服务器，runnerName，groupName 表示 Job 执行者信息和 Task 分组名称
//taskId 表示 Job 的 Id 主键，即数据库 id 字段。status 表示 Job 的状态，一般是 FINISHED，note 一般为空，可以附带信息。
func FinishJob(runnerName, groupName string, taskId int, status, result, note string) (FinishedTask, error) {
	finishJobURL := HuginnBaseUrl + "/job/" + groupName + "/finish"
	var param string
	if note == "" {
		param = fmt.Sprintf("runner=%s&taskId=%d&status=%s&result=%s",
			runnerName,taskId,strings.ToUpper(status),url.PathEscape(result))
	} else {
		param = fmt.Sprintf("runner=%s&taskId=%d&status=%s&result=%s&note=%s",
			runnerName,taskId,strings.ToUpper(status),url.PathEscape(result),url.PathEscape(note))
	}
	cookie, err := login()
	if err != nil {
		printErr(err)
		return FinishedTask{}, err
	}
	client := http.Client{}
	allURL := finishJobURL+"?"+param
	//println(allURL)
	request, err := http.NewRequest("POST", allURL, nil)
	if err != nil {
		printErr(err)
		return FinishedTask{}, err
	}
	request.AddCookie(&cookie)
	do, err := client.Do(request)
	if err != nil {
		printErr(err)
		return FinishedTask{}, err
	}
	all, _ := io.ReadAll(do.Body)
	var finishedTask FinishedTask
	err = json.Unmarshal(all, &finishedTask)
	if err != nil {
		printErr(err)
		log.Println("Error with Unmarshal", string(all))
		return FinishedTask{}, err
	}
	return finishedTask, err
}
