package HappyHelper

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"net/http"
)

type HappyParser interface {
	Parse(doc *goquery.Document) error
	Export(path string) error
}

type PageSnapShot struct {
	Client *http.Client
	UA     string
	body   io.ReadCloser
	Parser HappyParser
}

func NewSnapShot(parser HappyParser) *PageSnapShot {
	return &PageSnapShot{
		Parser: parser,
		Client: &http.Client{},
	}
}

func (p *PageSnapShot) requestPage(src string) (*goquery.Document, error) {
	req, err := http.NewRequest("GET", src, nil)
	if err != nil {
		return nil, err
	}
	ua := p.UA
	if len(ua) == 0 {
		ua = DefaultUA
	}
	client := p.Client
	if client == nil {
		client = &http.Client{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	p.body = resp.Body
	return goquery.NewDocumentFromReader(resp.Body)
}

func (p *PageSnapShot) Parse(src string) error {
	if p.Parser == nil || len(src) == 0 {
		return fmt.Errorf("no parser for %v", src)
	}
	doc, err := p.requestPage(src)
	defer func() {
		if p.body != nil {
			_ = p.body.Close()
		}
	}()
	if err != nil {
		return err
	}
	return p.Parser.Parse(doc)
}

func (p *PageSnapShot) Download(dst string) error {
	if p.Parser == nil || len(dst) == 0 {
		return fmt.Errorf("no parser for path: %v", dst)
	}
	return p.Parser.Export(dst)
}
