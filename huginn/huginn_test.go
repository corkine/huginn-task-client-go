package huginn

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/encoding/simplifiedchinese"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

//const CHECK_URL = "http://www.nyzsb.com.cn/wwwnew/datacenter/zzcjcx/"
const CHECK_URL = "http://218.28.109.125:18080/datacenter/zzcjcx/"
var GROUP_NAME = "zk21"
var PROMISE_RETURN_SECONDS = 100

var queryURL = func(idNumber string, allowNumber string) string {
	return fmt.Sprintf("txtZKZH=%s&txtBMXH=&txtSFZH=%s&%s",
		allowNumber, idNumber[len(idNumber) - 4 :],
		url.PathEscape("SubButton.x=43&SubButton.y=11"))
}

type Charset string

const (
	UTF8    = Charset("UTF-8")
	GB18030 = Charset("GB18030")
)

func ConvertByte2String(byte []byte, charset Charset) string {
	var str string
	switch charset {
	case GB18030:
		var decodeBytes,_=simplifiedchinese.GB18030.NewDecoder().Bytes(byte)
		str= string(decodeBytes)
	case UTF8:
		fallthrough
	default:
		str = string(byte)
	}
	return str
}

func fuckRun(runnerName string) {
	defer func() {
		log.Println("Releasing lock")
		<-flow
	}()
	flow <- 1
	log.Printf("%s Fetching next job...",runnerName)
	job, err := FetchJob(runnerName, GROUP_NAME, 1, PROMISE_RETURN_SECONDS)
	if err != nil {
		printErr(err)
		return
	}
	if len(job) < 0 {
		log.Printf("Error fetch job from huginnServer %#v",job)
		return
	}
	task := job[0]
	data := task.Data
	log.Printf("Starting handle %#v", data)
	//data := "9年级15班 马飞扬 411329200505290676 13291720228799 211329109599"
	split := strings.Split(data, " ")
	if len(split) != 5 || split[2] == "" || split[4] == "" {
		printErr(fmt.Errorf("can't parse data %s", data))
		return
	}
	println(split)
	clazz,userName,idNumber,checkNumber,allowNumber := split[0],split[1],split[2],split[3],split[4]
	client := http.Client{}
	checkURL := CHECK_URL
	log.Println("Checking " + checkURL)
	request, err := http.NewRequest("POST",checkURL,
		strings.NewReader(queryURL(idNumber,allowNumber)))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type","application/x-www-form-urlencoded")
	request.Header.Set("Accept","text/html,application/xhtml+xml,application/xml;q=0.9," +
		"image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9")
	request.Header.Set("Accept-Encoding","gzip, deflate")
	request.Header.Set("Content-Length","72")
	request.Header.Set("User-Agent",
		"User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) " +
		"Chrome/91.0.4472.124 Safari/537.36 Edg/91.0.864.64")
	request.Header.Set("Referer","http://www.nyzsb.com.cn/wwwnew/datacenter/zzcjcx/")
	request.Header.Set("Host","www.nyzsb.com.cn")
	request.Header.Set("Accept-Language","zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6,ja;q=0.5")
	request.Header.Set("Origin","http://www.nyzsb.com.cn")
	post, err := client.Do(request)
	/*post, err := http.PostForm("http://www.nyzsb.com.cn/wwwnew/datacenter/zzcjcx/", map[string][]string{
		"txtZKZH": {allowNumber},
		"txtBMXH": {""},
		"txtSFZH": {idNumber},
	})*/
	if err != nil {
		printErr(err)
		return
	}
	all, err := io.ReadAll(post.Body)

	if err != nil {
		printErr(err)
		return
	}
	//println(ConvertByte2String(all,GB18030))
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(ConvertByte2String(all, GB18030)))
	if err != nil {
		printErr(err)
		return
	}
	name := doc.Find("body > table > tbody > tr:nth-child(2) > td.common").Text()
	name = strings.TrimSpace(name)
	sumScore := doc.Find("body > table > tbody > tr:nth-child(3) > td.common").Text()
	sumScore = strings.TrimSpace(sumScore)
	chinese := doc.Find("body > table > tbody > tr:nth-child(4) > td.common").Text()
	chinese = strings.TrimSpace(chinese)
	math := doc.Find("body > table > tbody > tr:nth-child(5) > td.common").Text()
	math = strings.TrimSpace(math)
	english := doc.Find("body > table > tbody > tr:nth-child(6) > td.common").Text()
	english = strings.TrimSpace(english)
	phy := doc.Find("body > table > tbody > tr:nth-child(7) > td.common").Text()
	phy = strings.TrimSpace(phy)
	che := doc.Find("body > table > tbody > tr:nth-child(8) > td.common").Text()
	che = strings.TrimSpace(che)
	policy := doc.Find("body > table > tbody > tr:nth-child(9) > td.common").Text()
	policy = strings.TrimSpace(policy)
	history := doc.Find("body > table > tbody > tr:nth-child(10) > td.common").Text()
	history = strings.TrimSpace(history)
	log.Printf("For %s, Sum %s, Chinese %s " +
		"math %s, english %s, phy %s, che %s, pol %s, his %s",name,sumScore,chinese,
		math,english,phy,che,policy,history)
	line := fmt.Sprintf("%s,%s,%s,%s,%s," +
		"%s,%s,%s,%s,%s,%s,%s,%s",
		clazz,userName,idNumber,checkNumber,allowNumber,
		sumScore,chinese,math,english,phy,che,policy,history)
	log.Println("Submitting job...")
	finishJob, err := FinishJob(runnerName, GROUP_NAME, task.Id, "FINISHED", line, "")
	if err != nil {
		printErr(err)
		log.Printf("Finished Job: %#v",finishJob)
		return
	}
	log.Printf("Job Finished %d with status %s ",finishJob.TaskId, finishJob.FinishStatus)
}

var flow = make(chan int, 10)

func main() {
	for {
		num := rand.Intn(99) + 200
		go fuckRun(fmt.Sprintf("RUNNER-%d",num))
		time.Sleep(time.Second * 2)
	}
}

func test() {
	/*status, err := GetStatus(GROUP_NAME, "ALL")
	if err != nil {
		return
	}
	fmt.Printf("%#v", status)*/
	/*d,_ := FetchJob("XIAOXIN_PRO","zk2021",1,100)
	fmt.Printf("%#v", d)
	t := d[0]
	f, _ := FinishJob("XIAOXIN_PRO","zk2021",t.Id,"FINISHED","SET RESULT","")
	fmt.Printf("%#v", f)*/
}