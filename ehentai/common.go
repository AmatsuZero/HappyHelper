package ehentai

import (
	"HappyHelper"
	"context"
	"github.com/PuerkitoBio/goquery"
	"github.com/reactivex/rxgo/v2"
	"net/http"
	"net/url"
)

func requestSinglePage(src string, client *http.Client) (*goquery.Document, error) {
	u, err := url.Parse(src)
	if err != nil {
		return nil, err
	}
	resp, err := client.Get(src)
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

func wrappedRequestSinglePage(src string, client *http.Client) rxgo.Observable {
	return rxgo.Create([]rxgo.Producer{func(ctx context.Context, next chan<- rxgo.Item) {
		doc, err := requestSinglePage(src, client)
		next <- rxgo.Item{
			V: doc,
			E: err,
		}
	}})
}

func retryStrategy(err error) bool {
	if err.Error() == HappyHelper.OverLimitError { // TODO：如果超出下载配额，可能下载链接都是509错误页，待解决
		return false
	} else if err.Error() == "no doc" {
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
