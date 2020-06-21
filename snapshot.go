package HappyHelper

import (
	"fmt"
)

type HappyParser interface {
	Parse(src string) error
	Export(path string) error
}

type PageSnapShot struct {
	Parser HappyParser
}

func NewSnapShot(parser HappyParser) *PageSnapShot {
	return &PageSnapShot{
		Parser: parser,
	}
}

func (p *PageSnapShot) Parse(src string) error {
	return p.Parser.Parse(src)
}

func (p *PageSnapShot) Download(dst string) error {
	if p.Parser == nil || len(dst) == 0 {
		return fmt.Errorf("no parser for path: %v", dst)
	}
	return p.Parser.Export(dst)
}
