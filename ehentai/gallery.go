package ehentai

import (
	"HappyHelper"
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/reactivex/rxgo/v2"
	"net/http"
	"net/url"
)

type Gallery struct {
	Client   *http.Client
	Src      string   // 地址
	Links    []string // 图片地址
	BookName string   // 标题
}

/// 从大图浏览页面找到图片地址
func (g *Gallery) createObservable() rxgo.Observable {
	return g.createPageLinks(g.Src).Map(func(ctx context.Context, i interface{}) (interface{}, error) {
		link := i.(string)
		resp, err := g.Client.Get(link)
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
		// 找到图片
		src, _ := doc.Find("div[id=i3] a img").First().Attr("src")
		if src == HappyHelper.OverLimitErrorPage { // 判断是否是509错误页
			err = fmt.Errorf("%v", HappyHelper.OverLimitError)
		}
		return src, err
	})
}

func (g *Gallery) Parse() rxgo.Disposed {
	item := g.createObservable()
	return item.ForEach(func(i interface{}) {
		if g.Links == nil {
			g.Links = make([]string, 0)
		}
		g.Links = append(g.Links, i.(string))
	}, func(err error) {
		fmt.Println("遇到错误: " + err.Error())
	}, func() {
		fmt.Printf("书名: %v, 共 %d 页\n", g.BookName, len(g.Links))
		fmt.Println("========================================")
	})
}

/// 找到当前页面一共有多少页， 获取每个页面的链接
func (g *Gallery) createPageLinks(src string) rxgo.Observable {
	fmt.Printf("正在解析: %v \n", src)
	item := wrappedRequestSinglePage(src, g.Client).Retry(3, retryStrategy)
	item = item.FlatMap(func(item rxgo.Item) rxgo.Observable {
		if item.E != nil {
			return rxgo.Just(item.E)()
		} else if item.V == nil {
			return rxgo.Just(fmt.Errorf("no doc"))()
		}
		doc := item.V.(*goquery.Document)
		// 找到标题
		g.BookName = doc.Find("head title").First().Text()
		return PagePathGallery.CreatePageLinks(doc).FlatMap(func(item rxgo.Item) rxgo.Observable {
			if item.E != nil {
				return rxgo.Just(item.E)()
			}
			link := item.V.(string)
			// 获取当前页码
			page, e := PagePathGallery.CurrentPage(link)
			if e != nil {
				page = 0 // 当成第1页
			}
			return g.parseSinglePage(page, link)
		})
	}, rxgo.WithBufferedChannel(10))
	return item
}

/// 找到页面里面的所有大图浏览页面链接
func (g *Gallery) parseSinglePage(page int, src string) rxgo.Observable {
	fmt.Printf("即将解析第%d页: %v\n", page+1, src)
	item := wrappedRequestSinglePage(src, g.Client).Retry(3, retryStrategy)
	item = item.FlatMap(func(item rxgo.Item) rxgo.Observable {
		if item.E != nil {
			return rxgo.Just(item.E)()
		}
		doc := item.V.(*goquery.Document)
		return rxgo.Just(doc.Find("div[id=gdt] div[class=gdtm]").Map(func(i int, selection *goquery.Selection) string {
			// div 元素，找到里面的 <a> 标签
			addr, _ := selection.Find("div a").First().Attr("href")
			return addr
		}))().Filter(func(i interface{}) bool { // 过滤掉无效的链接
			s, ok := i.(string)
			return ok && len(s) > 0
		})
	})
	return item
}

func (g Gallery) Download(dst string) <-chan struct{} {
	tmpDir, e := g.TmpDir()
	if e != nil {
		return nil
	}
	item := rxgo.Just(g.Links)().FlatMap(func(item rxgo.Item) rxgo.Observable {
		return g.download(item.V.(string), tmpDir)
	})
	return item.ForEach(func(i interface{}) {
		fmt.Printf("图片: %v 下载完成\n", i)
	}, func(err error) {
		fmt.Printf("下载失败: %v\n", err)
	}, func() { // 全部下载结束，创建压缩包，并移动到指定目录
		defer func() {
			HappyHelper.CleanAllTmpDirs([]string{tmpDir})
			fmt.Println("导出完成")
		}()
		e := HappyHelper.ZipFiles(dst, []string{tmpDir})
		if e != nil {
			fmt.Printf("创建压缩包失败: %v\n", e)
		}
	})
}

/// 下载图片到指定指定文件夹
func (g *Gallery) download(src, dst string) rxgo.Observable {
	item := rxgo.Defer([]rxgo.Producer{func(ctx context.Context, next chan<- rxgo.Item) {
		addr, err := url.Parse(src)
		if err != nil {
			next <- rxgo.Error(err)
			return
		}
		req, err := http.NewRequest("GET", addr.String(), nil)
		if err != nil {
			next <- rxgo.Error(err)
			return
		}
		req.Header.Set("Connection", "close") // 尝试解决 too many connections
		resp, err := g.Client.Do(req)
		if err != nil {
			next <- rxgo.Error(err)
			return
		}
		defer func() { // TODO：如果超出限额，这里会重定向到这样的页面：https://pabdsvx.njxanimfdxzh.hath.network/h/80，可能会导致Crash，
			_ = resp.Body.Close()
		}()
		fileName := HappyHelper.GetFileName(addr)
		output, err := HappyHelper.PutFile(fileName, dst, resp.Body)
		next <- rxgo.Item{
			V: output,
			E: err,
		}
	}})
	return item.Retry(3, retryStrategy)
}

func (g *Gallery) TmpDir() (string, error) {
	return HappyHelper.MKTmpDirIfNotExist(g.BookName)
}
