package ehentai

import (
	"HappyHelper"
	"context"
	"encoding/json"
	"fmt"
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

func (mode DisplayMode) findPages(doc *goquery.Document) rxgo.Observable {
	item := rxgo.Just(doc.Find(mode.patternString()).Map(func(i int, selection *goquery.Selection) string {
		src, _ := selection.Attr("href")
		return src
	}))()
	item = item.Filter(func(i interface{}) bool {
		s, ok := i.(string)
		return ok && len(s) > 0
	}).Distinct(func(ctx context.Context, i interface{}) (interface{}, error) { // 去重
		return i, nil
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
	case strings.Contains(src.Path, "/g/"):
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

/// 分页字符串
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

/// 根据页面类型，找到当前页码
func (t PagePathType) CurrentPage(src string) (int, error) {
	u, e := url.Parse(src)
	if e != nil {
		return 0, e
	}
	switch t {
	case PagePathTag:
		paths := HappyHelper.NewPaths(u)
		return strconv.Atoi(paths.LastComponent())
	default:
		return strconv.Atoi(u.Query().Get(t.PageQuery()))
	}
}

/// 根据当前页面类型，找到最大页码
func (t PagePathType) FindMaxPage(doc *goquery.Document) (rxgo.Item, error) {
	return rxgo.Just(doc.Find(t.PagePattern()).Map(func(i int, selection *goquery.Selection) string {
		src, _ := selection.Attr("href")
		return src
	}))().Filter(func(i interface{}) bool {
		s, ok := i.(string)
		return ok && len(s) > 0
	}).Map(func(ctx context.Context, i interface{}) (interface{}, error) {
		p, e := t.CurrentPage(i.(string))
		if e != nil {
			return 0, nil
		}
		return p, nil
	}).Max(func(i interface{}, i2 interface{}) int {
		return i.(int) - i2.(int)
	}).Get()
}

/// 根据当前页面最大页码，创建链接
func (t PagePathType) CreatePageLinks(doc *goquery.Document) rxgo.Observable {
	maxPage, err := t.FindMaxPage(doc)
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
	}).Distinct(func(ctx context.Context, i interface{}) (interface{}, error) { // 去重
		return i, err
	})
	if start == maxPage.V.(int) {
		return item.Take(1)
	}
	return item
}

type TagType int

const (
	HTagUnknown TagType = iota
	HTagGroup
	HTagArtist
	HTagMale
	HTagFemale
	HTagMisc
)

func NewHTag(tag string) TagType {
	switch tag {
	case "group":
		return HTagGroup
	case "artist":
		return HTagArtist
	case "male":
		return HTagMale
	case "female":
		return HTagFemale
	case "misc":
		return HTagMisc
	default:
		return HTagUnknown
	}
}

func (t TagType) TableName() string {
	switch t {
	case HTagGroup:
		return "ehentai_group"
	case HTagArtist:
		return "ehentai_artist"
	case HTagFemale:
		return "ehentai_female"
	case HTagMale:
		return "ehentai_male"
	case HTagMisc:
		return "ehentai_misc"
	default:
		return ""
	}
}

func (t TagType) CreateTableQuery() string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %v (
		'id' INTEGER PRIMARY KEY AUTOINCREMENT,
		'value' TEXT NOT NULL UNIQUE,
		'links' TEXT NULL
	)`, t.TableName())
}

type Tags struct {
	Type  TagType
	Name  string
	Links []string
}

func (t *Tags) insert() (int64, error) {
	urlsJSON, err := json.Marshal(t.Links)
	if err != nil {
		return 0, err
	}
	return DefaultSerializeManager.Insert(t.Type.TableName(), map[string]interface{}{
		"value": t.Name,
		"links": string(urlsJSON),
	})
}

func (t *Tags) update() (int64, error) {
	urlsJSON, err := json.Marshal(t.Links)
	if err != nil {
		return 0, err
	}
	return DefaultSerializeManager.Update(t.Type.TableName(), map[string]interface{}{
		"value": t.Name,
		"links": string(urlsJSON),
	}, map[string]interface{}{
		"value": t.Name,
	})
}

func (t TagType) RemoveLinksFromAllTag(src string) (int64, error) {
	db, err := DefaultSerializeManager.GetDB()
	if err != nil {
		return 0, err
	}
	rows, err := db.Query(fmt.Sprintf("SELECT id from %v WHERE instr(links, ?) > 0", t.TableName()), src)
	if err != nil {
		return 0, err
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id int
		err = rows.Scan(&id)
		if err != nil {
			return 0, err
		}
		ids = append(ids, strconv.Itoa(id))
	}
	err = rows.Close()
	if err != nil {
		return 0, err
	}
	query := fmt.Sprintf("DELETE from %v where id in(%v)", t.TableName(), strings.Join(ids, ","))
	stmt, err := db.Prepare(query)
	if err != nil {
		return 0, err
	}
	ret, err := stmt.Exec()
	if err != nil {
		return 0, err
	}
	return ret.RowsAffected()
}

func (t TagType) RemoveLinks(tagName string, src []string) (int64, error) {
	tag, err := t.restore(tagName)
	if err != nil || tag == nil {
		return 0, err
	}
	dict := make(map[string]int)
	for i, link := range src {
		dict[link] = i
	}
	newLinks := make([]string, 0)
	for _, link := range tag.Links {
		if _, ok := dict[link]; !ok {
			newLinks = append(newLinks, link)
		}
	}
	tag.Links = newLinks
	return tag.update()
}

func (t TagType) restore(tagName string) (tag *Tags, err error) {
	err = DefaultSerializeManager.CreateTable(t.CreateTableQuery())
	if err != nil {
		return nil, err
	}
	rows, err := DefaultSerializeManager.GetRows(t.TableName(), map[string]interface{}{
		"value": tagName,
	})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	for rows.Next() {
		var id int
		var value string
		var links string
		err = rows.Scan(&id, &value, &links)
		if err != nil {
			return nil, err
		}
		tmp := &Tags{
			Type: t,
			Name: tagName,
		}
		err = json.Unmarshal([]byte(links), &tmp.Links)
		if err != nil {
			return nil, err
		}
		tag = tmp
	}
	return tag, err
}

func (t TagType) UpdateLinks(tagName string, src []string) (int64, error) {
	tag, err := t.restore(tagName)
	if err != nil {
		return 0, err
	}
	if tag == nil {
		tag = &Tags{
			Type:  t,
			Name:  tagName,
			Links: src,
		}
		return tag.insert()
	}
	dict := make(map[string]int)
	for i, link := range tag.Links {
		dict[link] = i
	}
	for _, link := range src {
		if _, ok := dict[link]; !ok {
			tag.Links = append(tag.Links, link)
		}
	}
	return tag.update()
}
