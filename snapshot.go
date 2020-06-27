package HappyHelper

import "fmt"

type HappyParser interface {
	Parse(src string) (<-chan struct{}, error)
	Export(path string) (<-chan struct{}, error)
}

type PageSnapShot struct {
	Parser HappyParser
}

func NewSnapShot(parser HappyParser) *PageSnapShot {
	return &PageSnapShot{
		Parser: parser,
	}
}

func (p *PageSnapShot) Parse(src string) (<-chan struct{}, error) {
	return p.Parser.Parse(src)
}

func (p *PageSnapShot) Download(dst string) (<-chan struct{}, error) {
	if p.Parser == nil || len(dst) == 0 {
		return nil, fmt.Errorf("输入错误")
	}
	return p.Parser.Export(dst)
}
