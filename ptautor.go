// @Author haonan3344@gmail.com
// 运行在群晖上，自动访问网页，下载free种子

package main

import (
	"bytes"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/PuerkitoBio/goquery"
	"github.com/parnurzeal/gorequest"
	"github.com/sirupsen/logrus"
	"github.com/thinkgos/meter"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"
)

type Config struct {
	CookieString  string
	UserAgent     string
	MinSize       string //单位 GB，最小下载大小
	MaxSize       string //单位 GB，最大下载大小
	CheckInterval uint   //检查间隔，单位秒
	RetryInterval uint   //错误重试间隔，单位秒
	TorrentsDir   string //种子保存位置

	minSizeVal uint64
	maxSizeVal uint64
}

var conf Config

var log = logrus.New()

func main() {
	readConfig()
	logInit()
	log.Info("PtAutor started")

	for {
		page, errs := getPage()
		if len(errs) == 0 {

			parse(page)
			log.Debug("slee   ping...")

			time.Sleep(time.Duration(conf.CheckInterval) * time.Minute)
		} else {
			for err := range errs {
				log.Errorf("get page error:%s", err)
			}
			time.Sleep(time.Duration(conf.RetryInterval) * time.Minute)
		}
	}
}

func readConfig() {
	var tomlData string

	var config_filename = "auto.toml"
	if btomlData, err := ioutil.ReadFile(config_filename); err != nil {
		fmt.Printf("open config file %s error!", config_filename)
		return
	} else {
		tomlData = string(btomlData)
	}

	if _, err := toml.Decode(tomlData, &conf); err != nil {
		fmt.Printf("parse config file %s error!", config_filename)
		return
	}
	conf.maxSizeVal, _ = meter.ParseBytes(conf.MaxSize)
	conf.minSizeVal, _ = meter.ParseBytes(conf.MinSize)

	// 检查配置值的有效性，并修正
	if conf.CheckInterval == 0 {
		conf.CheckInterval = 1
	}
	if conf.RetryInterval == 0 {
		conf.RetryInterval = 1
	}
}

func logInit() {
	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&logrus.JSONFormatter{})

	file, err := os.OpenFile("ptautor.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(file)
	} else {
		log.SetOutput(os.Stdout)
		log.Info("Failed to log to file, using default stderr")
	}

	// Only log the warning severity or above.
	log.SetLevel(logrus.DebugLevel)
}

func getPage() (string, []error) {
	log.Debug("getPage...")
	req := gorequest.New()
	_, body, errs := req.Get("https://www.hdarea.co/torrents.php").
		Set("User-Agent", conf.UserAgent).
		Set("Cookie", conf.CookieString).End()

	return string(body), errs
}

func parse(page string) {
	log.Debug("parse...")
	//file,_:=os.Open("torrents.php.txt")
	//doc,_:=goquery.NewDocumentFromReader(bufio.NewReader(file))
	doc, _ := goquery.NewDocumentFromReader(bytes.NewReader([]byte(page)))
	// fmt.Println("ignore: i, id,top, bookmark, free, to_download, name, stime, size")
	doc.Find("#outer > table > tbody > tr > td > table:nth-child(11) > tbody > tr").Each(func(i int, s *goquery.Selection) {
		//标题行忽略
		if i == 0 {
			return
		}
		name := s.Find("  td:nth-child(1) > a > b").Text()
		//日期
		stime := s.Find(" td:nth-child(4)").Text()
		//大小
		size := s.Find(" td:nth-child(5)").Text()

		//获取状态，是否已经收藏
		id_herf := s.Find("td:nth-child(2) > table > tbody > tr > td:nth-child(1) > a").AttrOr("href", "")
		id := strings.Trim(id_herf, "details.php?id=")
		id = strings.Split(id, "&")[0]
		bookmark := false

		sbookmark, _ := s.Find("td:nth-child(2) > table > tbody > tr > td:nth-child(4)").Html()
		bookmark = strings.Contains(sbookmark, "Bookmarked")

		//是否是free
		free := 0
		title_col, _ := s.Find("td:nth-child(2) > table > tbody > tr > td:nth-child(1)").Html()
		if strings.Contains(title_col, "Free") {
			free = 1
		}
		if strings.Contains(title_col, "2X Free") {
			free = 2
		}

		//是否置顶
		top := false
		if strings.Contains(title_col, "置顶") {
			top = true
		}

		to_download := true

		//进行条件判断分析
		//首先要free
		if free == 0 {
			to_download = false
		}

		//时间需要在一小时内，即 stime 不含 时 天 月 年
		if to_download && !top && (strings.Contains(stime, "时") || strings.Contains(stime, "天") || strings.Contains(stime, "月") || strings.Contains(stime, "年")) {
			to_download = false
		}

		//大小要在配置允许的范围内
		to_download_size_val, _ := meter.ParseBytes(size)
		// fmt.Println(meter.HumanSize(to_download_size_val))
		if to_download_size_val < conf.minSizeVal || to_download_size_val > conf.maxSizeVal {
			to_download = false
		}

		//还需要是未加收藏的
		if bookmark {
			to_download = false
		}

		if to_download {
			log.Info("Add bookmark:=======", i, id, top, bookmark, free, to_download, name, stime, size)
			addBookmark(id)
			downloadTorrent(id)

		} else {
			// fmt.Println("ignore:",i, id,top, bookmark, free, to_download, name, stime, size)
		}

	})
	//fmt.Println(torrents.Html())

}

func addBookmark(id string) {
	log.Debug("addBookmark...")
	url := "https://www.hdarea.co/bookmark.php?torrentid=" + id
	req := gorequest.New()
	_, body, _ := req.Get(url).
		Set("User-Agent", conf.UserAgent).
		Set("If-Modified-Since", time.Now().UTC().Format(http.TimeFormat)).
		Set("Cookie", conf.CookieString).End()

	log.Info("add bookmark ", id, string(body))
}

func delBookmark(id string) {
	log.Debug("addBookmark...")
	url := "https://www.hdarea.co/bookmark.php?torrentid=" + id
	req := gorequest.New()
	_, body, _ := req.Get(url).
		Set("User-Agent", conf.UserAgent).
		Set("If-Modified-Since", time.Now().UTC().Format(http.TimeFormat)).
		Set("Cookie", conf.CookieString).End()

	log.Info("undo bookmark ", id, string(body))
}

func downloadTorrent(id string) error {
	log.Debugf("downloadTorrent %s ...", id)
	url := "https://www.hdarea.co/download.php?id=" + id
	// Declare http client
	client := &http.Client{}

	// Declare HTTP Method and Url
	req, err := http.NewRequest("GET", url, nil)

	// Set cookie
	req.Header.Set("Cookie", conf.CookieString)
	req.Header.Set("User-Agent", conf.UserAgent)
	resp, err := client.Do(req)

	//从resp的字段中取得文件名
	contentDisposition := resp.Header.Get("Content-Disposition")
	_, params, err := mime.ParseMediaType(contentDisposition)
	filename := params["filename"] // set to "foo.png"

	// error handle
	if err != nil {
		log.Errorf("downloadTorrent error = %s \n", err)
	}

	// Print response
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(conf.TorrentsDir + "/" + filename)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)

	log.Infof("torrent %s: %s downloaded!", id, filename)

	return err
}
