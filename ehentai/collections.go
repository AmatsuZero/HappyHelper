package ehentai

import (
	"HappyHelper"
	"context"
	"github.com/PuerkitoBio/goquery"
	"github.com/reactivex/rxgo/v2"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type DisplayMode int

const (
	DisplayModeUnknown DisplayMode = iota
	DisplayModeMinimal
	DisplayModeMinimalPlus
	DisplayModeCompact
	DisplayModeExtended
	DisplayModeThumbnail
)

func NewDisplayMode(t string) DisplayMode {
	switch t {
	case "Minimal":
		return DisplayModeMinimal
	case "Minimal+":
		return DisplayModeMinimalPlus
	case "Compact":
		return DisplayModeCompact
	case "Extended":
		return DisplayModeExtended
	case "Thumbnail":
		return DisplayModeThumbnail
	default:
		return DisplayModeUnknown
	}
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

func (mode DisplayMode) findPages(doc *goquery.Document, eh *EHParser) rxgo.Observable {
	item := rxgo.Just(doc.Find(mode.patternString()).Map(func(i int, selection *goquery.Selection) string {
		src, _ := selection.Attr("href")
		return src
	}))().Retry(eh.RetryCount, eh.retryStrategy)
	item = item.Filter(func(i interface{}) bool {
		s, ok := i.(string)
		return ok && len(s) > 0
	})
	return item
}

type PagePathType int

const (
	PagePathIndex   PagePathType = iota // 首页
	PagePathTag                         // 合集
	PagePathSearch                      // 搜索
	PagePathGallery                     // 画廊
)

func NewPagePathType(src *url.URL) PagePathType {
	switch {
	case strings.Contains(src.Path, "/tag/"):
		return PagePathTag
	case src.Query().Get("f_search") != "":
		return PagePathSearch
	case strings.Contains(src.Path, "/p/"):
		return PagePathGallery
	case src.Path == "/":
		return PagePathIndex
	default:
		return PagePathIndex
	}
}

func (t PagePathType) PagePattern() string {
	switch t {
	case PagePathSearch:
		return "div[class=ido] table[class=ptt] td a"
	case PagePathTag:
		return "div[class=ido] table[class=ptt] tbody tr td a"
	case PagePathGallery:
		return "div[class=gtb] td a"
	case PagePathIndex:
		return "table[class=ptt] td a"
	default:
		return ""
	}
}

func (t PagePathType) PageQuery() string {
	switch t {
	case PagePathGallery:
		return "p"
	case PagePathSearch:
		return "f_search"
	default:
		return "page"
	}
}

func (t PagePathType) CreatePageLinks(doc *goquery.Document) rxgo.Observable {
	maxPage, err := rxgo.Just(doc.Find(t.PagePattern()).Map(func(i int, selection *goquery.Selection) string {
		src, _ := selection.Attr("href")
		return src
	}))().Filter(func(i interface{}) bool {
		s, ok := i.(string)
		if !ok || len(s) == 0 {
			return false
		}
		if _, err := strconv.Atoi(s); err != nil { // 过滤掉无法转换成数字的
			return false
		}
		return true
	}).Map(func(ctx context.Context, i interface{}) (interface{}, error) {
		addr, err := url.Parse(i.(string))
		if err != nil {
			return 0, err
		}
		p := addr.Query().Get(t.PageQuery())
		return strconv.Atoi(p)
	}).Max(func(i interface{}, i2 interface{}) int {
		return i.(int) - i2.(int)
	}).Get()

	start := 0
	if err != nil {
		return rxgo.Just(err)()
	} else if maxPage.V == nil {
		if t == PagePathTag {
			maxPage.V, start = 1, 1 // tag 由于是拼接path，给一个默认值 1
		} else {
			maxPage.V, start = 0, 0
		}
	}
	baseURL := doc.Url.Scheme + "://" + path.Join(doc.Url.Host, doc.Url.Path)
	item := rxgo.Range(start, maxPage.V.(int)).Map(func(ctx context.Context, i interface{}) (interface{}, error) {
		u, e := url.Parse(baseURL)
		if e != nil {
			return nil, e
		}
		// tag 页面不太一样, 是通过追加 path，而不是 query 参数, 需要单独处理
		if t == PagePathTag {
			paths := HappyHelper.NewPaths(u)
			// 查看最后的Path是不是已经有数字了
			if _, err := strconv.Atoi(paths.LastComponent()); err == nil {
				paths.RemoveLast()
			}
			if i.(int) > start {
				paths.Append(strconv.Itoa(i.(int)))
			}
			u.Path = paths.Encode()
		} else {
			queries := u.Query()
			queries.Del(t.PageQuery())
			if i.(int) > start {
				queries.Set(t.PageQuery(), strconv.Itoa(i.(int)))
			}
			u.RawQuery = queries.Encode()
		}
		return u.String(), nil
	})
	if start == maxPage.V.(int) {
		return item.Take(1)
	}
	return item
}
