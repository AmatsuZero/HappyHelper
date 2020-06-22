package HappyHelper

import (
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/reactivex/rxgo/v2"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type DisplayMode int

const (
	DisplayModeUnknown             = iota
	DisplayModeMinimal DisplayMode = iota
	DisplayModeMinimalPlus
	DisplayModeCompact
	DisplayModeExtended
	DisplayModeThumbnail
)

func NewDisplayMode(t string) (m DisplayMode) {
	switch t {
	case "Minimal":
		m = DisplayModeMinimal
	case "Minimal+":
		m = DisplayModeMinimalPlus
	case "Compact":
		m = DisplayModeCompact
	case "Extended":
		m = DisplayModeExtended
	case "Thumbnail":
		m = DisplayModeThumbnail
	default:
		m = DisplayModeUnknown
	}
	return
}

func (mode DisplayMode) patternString() string {
	switch mode {
	case DisplayModeMinimal:
	case DisplayModeMinimalPlus:
		return "table[class='itg gltm'] td[class='gl3m glname'] a"
	case DisplayModeCompact:
		return "table[class='itg gltc'] td[class='gl3c glname'] a"
	case DisplayModeExtended:
		return "table[class='itg glte'] td[class=gl3e]+a"
	case DisplayModeThumbnail:
		return "table[class='itg gltd'] div[class=gl1t] div[class=gl3t] a"
	default:
		return ""
	}
	return ""
}

func (mode DisplayMode) findPages(doc *goquery.Document) rxgo.Observable {
	item := rxgo.Create([]rxgo.Producer{func(ctx context.Context, next chan<- rxgo.Item) {
		doc.Find(mode.patternString()).Each(func(i int, selection *goquery.Selection) {
			src, _ := selection.Attr("href")
			next <- rxgo.Of(src)
		})
	}})
	item = item.Filter(func(i interface{}) bool {
		s, ok := i.(string)
		return ok && len(s) > 0
	})
	return item
}

type EHParser struct {
	client *http.Client
	links  map[string]string
}

func NewEHParser() *EHParser {
	return &EHParser{
		client: &http.Client{},
		links:  make(map[string]string),
	}
}

func (eh *EHParser) Parse(src string) error {
	u, err := url.Parse(src)
	if err != nil {
		return err
	}
	fmt.Println("开始解析：" + u.String())
	pathSegments := strings.Split(u.Path, "/")
	set := NewOrderStringSet(pathSegments)
	if set.Contains("g") { // 表明是画廊模式
		return eh.parseGallery(u.String())
	}
	// 按照合集去解析
	return eh.parseGrid(src)
}

func (eh *EHParser) parseGrid(src string) error {
	fmt.Println("解析合集：" + src)
	doc, err := eh.requestSinglePage(src)
	if err != nil {
		return err
	}
	// 找到当前的展示模式
	pattern := "div[id=dms] select option[selected]"
	displayMode := NewDisplayMode(doc.Find(pattern).First().Text())
	for item := range displayMode.findPages(doc).Observe() {
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

func createPageLinks(doc *goquery.Document) rxgo.Observable {
	maxPage, err := rxgo.Create([]rxgo.Producer{func(ctx context.Context, next chan<- rxgo.Item) {
		doc.Find("div[class=gtb] td[onclick] a").Each(func(i int, selection *goquery.Selection) {
			src, _ := selection.Attr("href")
			next <- rxgo.Of(src)
		})
	}}).Filter(func(i interface{}) bool {
		s, ok := i.(string)
		return ok && len(s) > 0
	}).Map(func(ctx context.Context, i interface{}) (interface{}, error) {
		addr, err := url.Parse(i.(string))
		if err != nil {
			return nil, err
		}
		p := addr.Query().Get("p")
		return strconv.Atoi(p)
	}).Max(func(i interface{}, i2 interface{}) int {
		return i.(int) - i2.(int)
	}).Get()
	if err != nil || maxPage.V == nil {
		return rxgo.Empty()
	}
	baseURL := doc.Url.Scheme + "://" + path.Join(doc.Url.Host, doc.Url.Path)
	return rxgo.Defer([]rxgo.Producer{func(ctx context.Context, next chan<- rxgo.Item) {
		for i := 0; i < maxPage.V.(int); i++ {
			u, e := url.Parse(baseURL)
			if e != nil {
				next <- rxgo.Error(e)
				return
			}
			queries := u.Query()
			queries.Set("p", strconv.Itoa(i))
			u.RawQuery = queries.Encode()
			next <- rxgo.Of(u.String())
		}
	}})
}

func (eh *EHParser) parseGallery(src string) error {
	fmt.Println("解析画廊：" + src)
	item, err := rxgo.Create([]rxgo.Producer{func(ctx context.Context, next chan<- rxgo.Item) {
		doc, err := eh.requestSinglePage(src)
		next <- rxgo.Item{
			V: doc,
			E: err,
		}
	}}).Retry(3, eh.retryStrategy).First().Get()
	if err != nil {
		return err
	}
	ob := createPageLinks(item.V.(*goquery.Document)).Map(func(ctx context.Context, i interface{}) (interface{}, error) {
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
	tmpDir, err := MKTmpDirIfNotExist(title)
	if err != nil {
		fmt.Printf("创建临时文件夹失败 %v\n", err)
		return
	}
	for item := range links.Observe() {
		if item.E != nil {
			fmt.Printf("获取图片地址失败 : %v\n", item.E)
			continue
		} else {
			eh.links[item.V.(string)] = tmpDir
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
	if src == OverLimitErrorPage { // 判断是否是509错误页
		err = fmt.Errorf("%v", OverLimitError)
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
		req.Header.Set("Connection", "close") // 尝试解决 too many connections
		resp, err := eh.client.Do(req)
		if err != nil {
			next <- rxgo.Error(err)
			return
		}
		defer func() { // TODO：如果超出限额，这里会重定向到这样的页面：https://pabdsvx.njxanimfdxzh.hath.network/h/80，可能会导致Crash，
			_ = resp.Body.Close()
		}()
		fileName := GetFileName(addr)
		output, err := PutFile(fileName, dst, resp.Body)
		next <- rxgo.Item{
			V: output,
			E: err,
		}
	}})
	return item.Retry(3, eh.retryStrategy)
}

func (eh *EHParser) retryStrategy(err error) bool {
	if err.Error() == OverLimitError { // TODO：如果超出下载配额，可能下载链接都是509错误页，待解决
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
	fmt.Printf("开始导出，共%d项\n", len(eh.links))
	if len(eh.links) == 0 {
		return fmt.Errorf("no link found")
	}
	obs := make([]rxgo.Observable, 0)
	for link, tmpDir := range eh.links {
		obs = append(obs, eh.downloadPic(link, tmpDir))
	}
	for file := range rxgo.Merge(obs).Observe() {
		if file.E != nil {
			fmt.Printf("Download Failed: %v\n", file.E)
		}
	}
	defer func() {
		CleanAllTmpDirs(eh.links)
		fmt.Println("导出完成")
	}()
	return ZipFiles(path, eh.links)
}
