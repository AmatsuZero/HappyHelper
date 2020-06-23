package ehentai

import (
	"HappyHelper"
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/reactivex/rxgo/v2"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
)

type EHParser struct {
	client     *http.Client
	links      map[string][]string
	RetryCount int // 重试次数
}

func NewEHParser() *EHParser {
	return &EHParser{
		client:     &http.Client{},
		links:      make(map[string][]string),
		RetryCount: 3,
	}
}

func (eh *EHParser) Parse(src string) error {
	u, err := url.Parse(src)
	if err != nil || u == nil {
		return err
	}
	fmt.Println("开始解析：" + u.String())
	pathType := NewPagePathType(u)
	if pathType == PagePathGallery { // 画廊模式
		return eh.parseGallery(u.String())
	}
	// 按照合集去解析
	return eh.parseGrid(src)
}

func (eh *EHParser) parseGrid(src string) error {
	fmt.Println("解析合集：" + src)
	item, err := eh.wrappedRequestSinglePage(src)
	if err != nil {
		return err
	}
	doc := item.V.(*goquery.Document)
	pathType := NewPagePathType(doc.Url)
	ob := pathType.CreatePageLinks(doc).Map(func(ctx context.Context, i interface{}) (interface{}, error) {
		return eh.requestSinglePage(i.(string)) // 请求每一个页面
	}).Retry(eh.RetryCount, eh.retryStrategy).FlatMap(func(item rxgo.Item) rxgo.Observable {
		if item.E != nil {
			return rxgo.Empty()
		}
		doc := item.V.(*goquery.Document)
		// 找到当前的展示方式
		pattern := "div[id=dms] select option[selected]"
		displayMode := NewDisplayMode(doc.Find(pattern).First().Text())
		return displayMode.findPages(doc, eh) // 找到当前页面的所有链接
	})
	for item := range ob.Observe() {
		link := item.V.(string)
		fmt.Println(link)
		if err := eh.parseGallery(link); err != nil {
			fmt.Printf("解析页面 %v 失败：%v", link, err)
		} else {
			fmt.Println("解析结束")
		}
		fmt.Println("=========================================")
	}
	return nil
}

func (eh *EHParser) wrappedRequestSinglePage(src string) (rxgo.Item, error) {
	return rxgo.Create([]rxgo.Producer{func(ctx context.Context, next chan<- rxgo.Item) {
		doc, err := eh.requestSinglePage(src)
		next <- rxgo.Item{
			V: doc,
			E: err,
		}
	}}).Retry(eh.RetryCount, eh.retryStrategy).First().Get()
}

func (eh *EHParser) parseGallery(src string) error {
	fmt.Println("解析画廊：" + src)
	item, err := eh.wrappedRequestSinglePage(src)
	if err != nil {
		return err
	}
	pathType := NewPagePathType(item.V.(*goquery.Document).Url)
	ob := pathType.CreatePageLinks(item.V.(*goquery.Document)).Map(func(ctx context.Context, i interface{}) (interface{}, error) {
		return eh.requestSinglePage(i.(string))
	}).Retry(3, eh.retryStrategy)
	for page := range ob.Observe() {
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

func (eh *EHParser) requestSinglePage(src string) (*goquery.Document, error) {
	u, err := url.Parse(src)
	if err != nil {
		return nil, err
	}
	resp, err := eh.client.Get(src)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	d, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}
	d.Url = u
	return d, err
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
	// 找到标题
	title := doc.Find("head title").First().Text()
	tmpDir, err := HappyHelper.MKTmpDirIfNotExist(title)
	if err != nil {
		fmt.Printf("创建临时文件夹失败 %v\n", err)
		return
	}
	for item := range links.Observe() {
		if item.E != nil {
			fmt.Printf("获取图片地址失败 : %v\n", item.E)
		} else {
			arr, ok := eh.links[tmpDir]
			if !ok {
				arr = make([]string, 0)
			}
			arr = append(arr, item.V.(string))
			eh.links[tmpDir] = arr
		}
	}
}

/// 访问页面并获取图片地址
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
	// 找到图片
	src, _ := doc.Find("div[id=i3] a img").First().Attr("src")
	if src == HappyHelper.OverLimitErrorPage { // 判断是否是509错误页
		err = fmt.Errorf("%v", HappyHelper.OverLimitError)
	}
	return src, err
}

/// 下载图片
func (eh *EHParser) downloadPic(src, dst string) rxgo.Observable {
	item := rxgo.Defer([]rxgo.Producer{func(ctx context.Context, next chan<- rxgo.Item) {
		addr, err := url.Parse(src)
		if err != nil {
			next <- rxgo.Error(err)
			return
		}
		fmt.Printf("正在下载: %v\n", addr.String())
		req, err := http.NewRequest("GET", addr.String(), nil)
		if err != nil {
			next <- rxgo.Error(err)
			return
		}
		//req.Header.Set("Connection", "close") // 尝试解决 too many connections
		resp, err := eh.client.Do(req)
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
	return item.Retry(3, eh.retryStrategy)
}

func (eh *EHParser) retryStrategy(err error) bool {
	fmt.Println("遇到错误: " + err.Error())
	if err.Error() == HappyHelper.OverLimitError { // TODO：如果超出下载配额，可能下载链接都是509错误页，待解决
		return false
	} else if e, ok := err.(*url.Error); ok {
		switch e.Err.Error() {
		case "Too many open connections": // TODO: 待解决
			return true
		case "EOF": // 可能超过了服务器的允许的连接数
			return false
		}
	}
	return true
}

func (eh *EHParser) Export(path string) error {
	fmt.Printf("开始导出，共%d本\n", len(eh.links))
	if len(eh.links) == 0 {
		return fmt.Errorf("no link found")
	}
	obs := make([]rxgo.Observable, 0, len(eh.links))
	for tmpDir, links := range eh.links {
		tmp := map[string][]string{tmpDir: links}
		obs = append(obs, rxgo.Just(tmp)().FlatMap(func(item rxgo.Item) rxgo.Observable {
			var links []string
			var dir string
			for k, v := range item.V.(map[string][]string) {
				links = v
				dir = k
			}
			fmt.Printf("书名: %v, 共 %d 页\n", filepath.Base(tmpDir), len(links))
			tmp := make([]rxgo.Observable, 0, len(links))
			for _, link := range links {
				tmp = append(tmp, eh.downloadPic(link, dir))
			}
			return rxgo.Merge(tmp)
		}))
	}
	for file := range rxgo.Merge(obs).Observe() {
		if file.E != nil {
			fmt.Printf("Download Failed: %v\n", file.E)
		}
	}
	defer func() {
		HappyHelper.CleanAllTmpDirs(eh.links)
		fmt.Println("导出完成")
	}()
	return HappyHelper.ZipFiles(path, eh.links)
}
