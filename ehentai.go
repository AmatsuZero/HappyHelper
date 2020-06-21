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
	"strconv"
)

type EHParser struct {
	client *http.Client
	links  map[string]bool
	title  string
	tmpDir string
}

func NewEHParser() *EHParser {
	return &EHParser{
		client: &http.Client{},
		links:  make(map[string]bool),
	}
}

func (eh *EHParser) Parse(src string) error {
	fmt.Println("开始解析：" + src)
	var doc *goquery.Document
	for item := range eh.requestSinglePage(src).Retry(3, eh.retryStrategy).First().Observe() {
		if item.E != nil {
			return item.E
		} else {
			doc = item.V.(*goquery.Document)
		}
	}
	obs := make(map[string]rxgo.Observable)
	// 解析当前页
	eh.parseSingleDetailPage(doc)
	// 检查其他页面
	doc.Find("div[class=gtb] td[onclick] a").Each(func(i int, selection *goquery.Selection) {
		if src, ok := selection.Attr("href"); ok {
			// 检查是否已经添加过
			_, ok = obs[src]
			if !ok {
				obs[src] = eh.requestSinglePage(src)
			}
		}
	})
	links := make([]rxgo.Observable, 0)
	for _, v := range obs {
		links = append(links, v)
	}
	for page := range rxgo.Merge(links).Observe() {
		if page.E != nil {
			fmt.Printf("请求详情页失败 : %v\n", page.E)
			continue
		} else {
			d := page.V.(*goquery.Document)
			eh.parseSingleDetailPage(d)
		}
	}
	return nil
}

func (eh *EHParser) parseDetailPage(src string) {

}

func (eh *EHParser) requestSinglePage(src string) rxgo.Observable {
	item := rxgo.Defer([]rxgo.Producer{func(ctx context.Context, next chan<- rxgo.Item) {
		resp, err := eh.client.Get(src)
		if err != nil {
			next <- rxgo.Error(err)
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()
		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			next <- rxgo.Error(err)
			return
		}
		u, _ := url.Parse(src)
		doc.Url = u
		next <- rxgo.Item{
			V: doc,
			E: err,
		}
	}})
	item.Retry(3, eh.retryStrategy)
	return item
}

func (eh *EHParser) parseSingleDetailPage(doc *goquery.Document) {
	p := doc.Url.Query()
	if len(p) == 0 {
		fmt.Printf("正在解析第1页: %v\n", doc.Url.String())
	} else {
		n, _ := strconv.Atoi(p.Get("p"))
		fmt.Printf("正在解析第%d页: %v\n", n+1, doc.Url.String())
	}
	// 找到页面的所有链接
	links := rxgo.Defer([]rxgo.Producer{func(ctx context.Context, next chan<- rxgo.Item) {
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
			fmt.Printf("获取图片地址失败 : %v\n", item.E)
			continue
		} else {
			eh.links[item.V.(string)] = true
		}
	}
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
	if src == OverLimitErrorPage { // 判断是否是509错误页
		err = fmt.Errorf("%v", OverLimitError)
	}
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
		fmt.Printf("正在下载: %v\n", addr.String())
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
	if err.Error() == OverLimitError { // TODO：如果超出下载配额，可能下载链接都是509错误页，待解决
		return false
	}
	return true
}

func (eh *EHParser) Export(path string) error {
	fmt.Printf("开始导出，共%d项\n", len(eh.links))
	if len(eh.links) == 0 {
		return fmt.Errorf("no link found")
	}
	obs := make([]rxgo.Observable, 0)
	for link, _ := range eh.links {
		obs = append(obs, eh.downloadPic(link, eh.tmpDir).Retry(3, eh.retryStrategy))
	}
	for file := range rxgo.Merge(obs).Observe() {
		if file.E != nil {
			fmt.Printf("Download Failed: %v\n", file.E)
		}
	}
	defer func() {
		if err := os.RemoveAll(eh.tmpDir); err != nil { // 删除临时文件夹
			fmt.Printf("删除临时文件夹失败 %v\n", err)
		}
		fmt.Println("导出完成")
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
	fmt.Println("正在创建压缩包")
	err := ZipDir(path, eh.tmpDir) // 创建压缩包
	if err != nil {
		return err
	}
	return err
}
