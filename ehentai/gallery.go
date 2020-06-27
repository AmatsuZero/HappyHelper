package ehentai

import (
	"HappyHelper"
	"context"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/reactivex/rxgo/v2"
	"net/http"
	"net/url"
	"time"
)

type Gallery struct {
	Client         *http.Client
	Src            string   // 地址
	Links          []string // 图片地址
	BookName       string   // 标题
	AlterName      string   // 别名
	SerializedDate int64    // 写入到数据库的时间戳
	ArtistTags     []string // 作者标签
	GroupTags      []string // 团体标签
	FemaleTags     []string // 女性标签
	MaleTags       []string // 男性标签
	MiscTags       []string // 杂项
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
	}).Distinct(func(ctx context.Context, i interface{}) (interface{}, error) {
		return i, nil
	})
}

func (g *Gallery) Parse() rxgo.Disposed {
	if len(g.Links) > 0 {
		return rxgo.Empty().Run()
	}
	item := g.createObservable()

	return item.ForEach(func(i interface{}) {
		if g.Links == nil {
			g.Links = make([]string, 0)
		}
		g.Links = append(g.Links, i.(string))
	}, func(err error) {
		fmt.Println("遇到错误: " + err.Error())
	}, func() {
		fmt.Println(g)
		fmt.Println("========================================")
		// 持久化
		_, err := g.Serialize()
		if err != nil {
			fmt.Printf("持久化失败: %v\n", err)
		}
		// 写入标签
		cancel := g.writeTags().Run()
		<-cancel
	}, rxgo.WithBufferedChannel(10))
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
		g.extractDetailInfo(doc)
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

func (g *Gallery) writeTags() rxgo.Observable {
	obs := make([]rxgo.Observable, 0, 5)
	obs = append(obs, rxgo.Just(g.GroupTags)().Map(func(ctx context.Context, i interface{}) (interface{}, error) {
		return HTagGroup.UpdateLinks(i.(string), []string{g.Src})
	}))
	obs = append(obs, rxgo.Just(g.ArtistTags)().Map(func(ctx context.Context, i interface{}) (interface{}, error) {
		return HTagArtist.UpdateLinks(i.(string), []string{g.Src})
	}))
	obs = append(obs, rxgo.Just(g.MaleTags)().Map(func(ctx context.Context, i interface{}) (interface{}, error) {
		return HTagMale.UpdateLinks(i.(string), []string{g.Src})
	}))
	obs = append(obs, rxgo.Just(g.FemaleTags)().Map(func(ctx context.Context, i interface{}) (interface{}, error) {
		return HTagFemale.UpdateLinks(i.(string), []string{g.Src})
	}))
	obs = append(obs, rxgo.Just(g.FemaleTags)().Map(func(ctx context.Context, i interface{}) (interface{}, error) {
		return HTagMisc.UpdateLinks(i.(string), []string{g.Src})
	}))
	return rxgo.Concat(obs)
}

/// 提取页面详情
func (g *Gallery) extractDetailInfo(doc *goquery.Document) {
	// 找到标题
	g.BookName = doc.Find("div[class=gm] div[id=gd2] h1[id=gn]").First().Text()
	// 找到副标题
	g.AlterName = doc.Find("div[class=gm] div[id=gd2] h1[id=gj]").First().Text()

	tagList := doc.Find("div[class=gm] div[id=taglist] tbody")
	// 找到 Group 标签
	g.GroupTags = tagList.Find("td[class=tc]:contains('group:')+td div a").Map(func(i int, selection *goquery.Selection) string {
		return selection.Text()
	})
	// 作者 Artist Tags 标签
	g.ArtistTags = tagList.Find("td[class=tc]:contains('artist:')+td div a").Map(func(i int, selection *goquery.Selection) string {
		return selection.Text()
	})
	// 男性 Male 标签
	g.MaleTags = tagList.Find("td[class=tc]:contains('male:')+td div a").Map(func(i int, selection *goquery.Selection) string {
		return selection.Text()
	})
	// 女性 Female 标签
	g.FemaleTags = tagList.Find("td[class=tc]:contains('female:')+td div a").Map(func(i int, selection *goquery.Selection) string {
		return selection.Text()
	})
	// 其他 Misc 标签
	g.MiscTags = tagList.Find("td[class=tc]:contains('misc:')+td div a").Map(func(i int, selection *goquery.Selection) string {
		return selection.Text()
	})
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
	}, rxgo.WithBufferedChannel(10))
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

/// 简表语句
func (g Gallery) CreteTableQuery() string {
	return `
	CREATE TABLE IF NOT EXISTS gallery (
    'id' INTEGER PRIMARY KEY AUTOINCREMENT,
	'src' TEXT NOT NULL UNIQUE,
    'name' TEXT NULL,
    'links' TEXT NULL,
	'date' DATE NULL,
	'alt_name' TEXT NULL,
	'group' TEXT NULL,
	'artist' TEXT NULL,
	'male' TEXT NULL,
	'female' TEXT NULL,
	'misc' TEXT NULL
	)`
}

/// 保存到数据库
func (g *Gallery) Serialize() (id int64, err error) {
	links, err := json.Marshal(g.Links)
	if err != nil {
		return -1, err
	}
	group, err := json.Marshal(g.GroupTags)
	if err != nil {
		return -1, err
	}
	male, err := json.Marshal(g.MaleTags)
	if err != nil {
		return -1, err
	}
	female, err := json.Marshal(g.FemaleTags)
	if err != nil {
		return -1, err
	}
	misc, err := json.Marshal(g.MiscTags)
	if err != nil {
		return -1, err
	}
	artist, err := json.Marshal(g.ArtistTags)
	if err != nil {
		return -1, err
	}
	// 建表
	err = DefaultSerializeManager.CreateTable(g.CreteTableQuery())
	if err != nil {
		return -1, err
	}
	dict := map[string]interface{}{
		"name":       g.BookName,
		"links":      string(links),
		"src":        g.Src,
		"alt_name":   g.AlterName,
		"group_tags": string(group),
		"female":     string(female),
		"misc":       string(misc),
		"male":       string(male),
		"artist":     string(artist),
	}
	if g.SerializedDate == 0 { // 说明没有创建过
		g.SerializedDate = time.Now().Unix()
		dict["date"] = g.SerializedDate
		id, err = DefaultSerializeManager.Insert("gallery", dict)
	} else { // 改为更新
		g.SerializedDate = time.Now().Unix()
		dict["date"] = g.SerializedDate
		id, err = g.update(dict)
	}
	return
}

/// 按照网址，恢复数据
func RestoreGallery(src string) (g *Gallery, err error) {
	// 建表
	err = DefaultSerializeManager.CreateTable(Gallery{}.CreteTableQuery())
	if err != nil {
		return nil, err
	}
	rows, err := DefaultSerializeManager.GetRows("gallery", map[string]interface{}{
		"src": src,
	})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	for rows.Next() {
		var links string
		var name, altName string
		var id int
		var date time.Time
		var src string
		var group_tags, artist, female, male, misc string

		err = rows.Scan(&id, &src, &name, &links, &date,
			&altName, &group_tags, &artist, &male, &female, &misc)
		if err != nil {
			return nil, err
		}
		tmp := &Gallery{
			Src:            src,
			SerializedDate: date.Unix(),
			BookName:       name,
			AlterName:      altName,
		}
		err = json.Unmarshal([]byte(links), &tmp.Links)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal([]byte(group_tags), &tmp.GroupTags)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal([]byte(artist), &tmp.ArtistTags)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal([]byte(male), &tmp.MaleTags)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal([]byte(female), &tmp.FemaleTags)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal([]byte(misc), &tmp.MiscTags)
		if err != nil {
			return nil, err
		}
		g = tmp
		break
	}
	if g != nil { // 检查是否过期
		t := time.Now().Unix()
		if time.Duration(t-g.SerializedDate) > 24*time.Hour {
			return nil, nil
		}
	}
	return
}

/// 更新数据
func (g *Gallery) update(dict map[string]interface{}) (int64, error) {
	affect, err := DefaultSerializeManager.Update("gallery", dict, map[string]interface{}{"src": g.Src})
	if err != nil {
		return 0, err
	}
	return affect, err
}

func (g *Gallery) Delete() error {
	_, err := DefaultSerializeManager.Delete("gallery", map[string]interface{}{
		"src": g.Src,
	})
	if err != nil {
		return err
	}
	// 从标签记录删除
	if _, err := HTagGroup.RemoveLinksFromAllTag(g.Src); err != nil {
		fmt.Println("团队标签删除失败")
	}
	if _, err := HTagArtist.RemoveLinksFromAllTag(g.Src); err != nil {
		fmt.Println("作家标签删除失败")
	}
	if _, err := HTagMale.RemoveLinksFromAllTag(g.Src); err != nil {
		fmt.Println("男性标签删除失败")
	}
	if _, err := HTagFemale.RemoveLinksFromAllTag(g.Src); err != nil {
		fmt.Println("女性标签删除失败")
	}
	if _, err := HTagMisc.RemoveLinksFromAllTag(g.Src); err != nil {
		fmt.Println("其他标签删除失败")
	}
	return nil
}

func (g *Gallery) String() string {
	str := fmt.Sprintf("书名：%v\n", g.BookName)
	str += fmt.Sprintf("别名: %v\n", g.AlterName)
	str += fmt.Sprintf("作者：%v\n", g.ArtistTags)
	str += fmt.Sprintf("团体: %v\n", g.GroupTags)
	str += fmt.Sprintf("女性: %v\n", g.FemaleTags)
	str += fmt.Sprintf("男性: %v\n", g.MaleTags)
	str += fmt.Sprintf("其他：%v", g.MiscTags)
	return str
}
