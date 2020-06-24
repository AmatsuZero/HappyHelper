package HappyHelper

type HappyParser interface {
	Parse(src string) <-chan struct{}
	Export(path string) <-chan struct{}
}

type PageSnapShot struct {
	Parser HappyParser
}

func NewSnapShot(parser HappyParser) *PageSnapShot {
	return &PageSnapShot{
		Parser: parser,
	}
}

func (p *PageSnapShot) Parse(src string) <-chan struct{} {
	return p.Parser.Parse(src)
}

func (p *PageSnapShot) Download(dst string) <-chan struct{} {
	if p.Parser == nil || len(dst) == 0 {
		return nil
	}
	return p.Parser.Export(dst)
}
