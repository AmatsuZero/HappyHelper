package ehentai

import (
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/reactivex/rxgo/v2"
	"net/http"
	"os"
)

type Page struct {
	Src       string
	Client    *http.Client
	Galleries []*Gallery
}

func (p *Page) Parse() rxgo.Disposed {
	item := wrappedRequestSinglePage(p.Src, p.Client).Retry(3, retryStrategy) // 请求当前页面，获取当前页面信息
	item = item.FlatMap(func(item rxgo.Item) rxgo.Observable {
		if item.E != nil {
			return rxgo.Just(item.E)()
		}
		doc := item.V.(*goquery.Document)
		pathType := NewPagePathType(doc.Url) // 根据页面类型和最大页码，请求所有的页面
		return pathType.CreatePageLinks(doc).Map(func(ctx context.Context, i interface{}) (interface{}, error) {
			return requestSinglePage(i.(string), p.Client)
		}).Retry(3, retryStrategy)
	})
	item = item.FlatMap(func(item rxgo.Item) rxgo.Observable { // 根据页面的展示模式，找到所有画廊链接
		if item.E != nil {
			return rxgo.Empty()
		}
		doc := item.V.(*goquery.Document)
		pattern := "div[id=dms] select option[selected]" // 找到当前的展示方式
		displayMode := NewDisplayMode(doc.Find(pattern).First().Text())
		return displayMode.findPages(doc) // 找到当前页面的所有链接
	}).Map(func(ctx context.Context, i interface{}) (interface{}, error) { // Map 成画廊对象
		return &Gallery{
			Client: p.Client,
			Src:    i.(string),
		}, nil
	})

	return item.ForEach(func(i interface{}) {
		if p.Galleries == nil {
			p.Galleries = make([]*Gallery, 0)
		}
		g := i.(*Gallery)
		cancel := g.Parse()
		<-cancel
		p.Galleries = append(p.Galleries, g)
	}, func(err error) {
		fmt.Println("遇到错误: " + err.Error())
	}, func() {

	})
}

func (p *Page) TmpDir() (string, error) {
	return os.TempDir(), nil
}

func (p *Page) Download(dst string) <-chan struct{} {
	return rxgo.Just(p.Galleries)().ForEach(func(i interface{}) {
		g := i.(*Gallery)
		fmt.Println("========================================")
		fmt.Printf("开始下载 %v\n", g.BookName)
		<-g.Download(dst)
	}, func(err error) {
		fmt.Printf("遇到错误 %v\n", err)
	}, func() {
		fmt.Println("全部导出成功！！！")
		fmt.Println()
	}, rxgo.WithBufferedChannel(3))
}

func (p Page) CreteTableQuery() string {
	return `
	CREATE TABLE IF NOT EXISTS page (
    'id' INTEGER PRIMARY KEY AUTOINCREMENT,
	'src' TEXT NOT NULL UNIQUE,
	)`
}
