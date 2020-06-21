package HappyHelper

import (
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/reactivex/rxgo/v2"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

type EHParser struct {
	client *http.Client
	links  []string
	title  string
	tmpDir string
}

func NewEHParser() *EHParser {
	return &EHParser{client: &http.Client{}}
}

func (eh *EHParser) Parse(doc *goquery.Document) error {
	// 找到页面的所有链接
	links := rxgo.Create([]rxgo.Producer{func(ctx context.Context, next chan<- rxgo.Item) {
		doc.Find("div[id=gdt] div[class=gdtm]").Each(func(i int, selection *goquery.Selection) {
			// div 元素，找到里面的 <a> 标签
			addr, _ := selection.Find("div a").First().Attr("href")
			next <- rxgo.Of(addr)
		})
	}}).Filter(func(i interface{}) bool { // 过滤掉无效的链接
		s, ok := i.(string)
		return ok && len(s) > 0
	})
	links = links.Map(eh.visitSubPage)       // 获取图片地址
	links = links.Retry(3, eh.retryStrategy) // 设置重试
	for item := range links.Observe() {
		if item.E != nil {
			fmt.Printf("获取图片地址失败 : %v", item.E)
			continue
		} else {
			eh.links = append(eh.links, item.V.(string))
		}
	}
	return nil
}

/// 访问页面并获取图片地址，这里为了避免IP被Ban，采用逐个访问的方式
func (eh *EHParser) visitSubPage(ctx context.Context, i interface{}) (interface{}, error) {
	link := i.(string)
	resp, err := eh.client.Get(link)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}
	// 找到标题
	if len(eh.title) == 0 {
		eh.title = doc.Find("head title").First().Text()
		eh.tmpDir, err = MKTmpDirIfNotExist(eh.title)
		if err != nil {
			return nil, err
		}
	}
	// 找到图片
	src, _ := doc.Find("div[id=i3] a img").First().Attr("src")
	return src, err
}

/// 下载图片，这里为了避免IP被Ban，采用逐个访问的方式
func (eh *EHParser) downloadPic(src, dst string) rxgo.Observable {
	return rxgo.Create([]rxgo.Producer{func(ctx context.Context, next chan<- rxgo.Item) {
		addr, err := url.Parse(src)
		if err != nil {
			next <- rxgo.Error(err)
			return
		}
		resp, err := eh.client.Get(addr.String())
		defer func() {
			_ = resp.Body.Close()
		}()
		if err != nil {
			next <- rxgo.Error(err)
			return
		}
		fileName := GetFileName(addr)
		output, err := PutFile(fileName, dst, resp.Body)
		next <- rxgo.Item{
			V: output,
			E: err,
		}
	}})
}

func (eh *EHParser) retryStrategy(err error) bool {
	return true
}

func (eh *EHParser) Export(path string) error {
	obs := make([]rxgo.Observable, len(eh.links))
	for i, link := range eh.links {
		obs[i] = eh.downloadPic(link, eh.tmpDir).Retry(3, eh.retryStrategy)
	}
	for file := range rxgo.Merge(obs).Observe() {
		if file.E != nil {
			fmt.Printf("Download Failed: %v\n", file.E)
		} else {

		}
	}
	defer func() {
		if err := os.RemoveAll(eh.tmpDir); err != nil { // 删除临时文件夹
			fmt.Printf("删除临时文件夹失败 %v\n", err)
		}
	}()
	// 分辨是文件还是文件, 如果是文件夹，创建文件夹
	if filepath.Ext(path) != "zip" {
		_, err := os.Stat(path)
		if os.IsNotExist(err) { // 检查输出文件夹是否存在
			err = os.MkdirAll(path, os.ModePerm)
		}
		if err != nil {
			return err
		}
		path = filepath.Join(path, eh.title+".zip")
	}
	// 创建压缩包
	err := ZipDir(path, eh.tmpDir)
	if err != nil {
		return err
	}
	return err
}
