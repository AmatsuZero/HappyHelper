package ehentai

import (
	"HappyHelper"
	"fmt"
	"github.com/reactivex/rxgo/v2"
	"net/http"
	"net/url"
)

type EHParser struct {
	client       *http.Client
	DownloadItem HappyHelper.DownloadType
}

func NewEHParser() *EHParser {
	return &EHParser{
		client: &http.Client{},
	}
}

func (eh *EHParser) Parse(src string) <-chan struct{} {
	u, err := url.Parse(src)
	if err != nil || u == nil {
		return rxgo.Empty().Run()
	}
	fmt.Println("开始解析：" + u.String())
	fmt.Println("++++++++++++++++++++++++++++++++++++++++")
	pathType := NewPagePathType(u)
	if pathType == PagePathGallery { // 画廊模式
		g := &Gallery{
			Client: eh.client,
			Src:    src,
		}
		eh.DownloadItem = g
		return g.Parse()
	}
	// 按照合集去解析
	return eh.parseGrid(src)
}

func (eh *EHParser) parseGrid(src string) rxgo.Disposed {
	page := &Page{
		Src:    src,
		Client: eh.client,
	}
	eh.DownloadItem = page
	return page.Parse()
}

func (eh *EHParser) Export(path string) <-chan struct{} {
	if eh.DownloadItem == nil {
		return nil
	}
	return eh.DownloadItem.Download(path)
}
