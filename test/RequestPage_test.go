package test

import (
	"HappyHelper"
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

func TestFetchPage(t *testing.T) {
	t.Log(os.Getenv("https_proxy"))
	parser := HappyHelper.NewEHParser()
	snapShot := HappyHelper.NewSnapShot(parser)
	err := snapShot.Parse("https://e-hentai.org/g/1665668/86e951dd52/")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(os.TempDir())
	myself, err := user.Current()
	if err != nil {
		t.Fail()
	}
	homedir := myself.HomeDir
	output := filepath.Join("Desktop", "output")
	err = snapShot.Download(filepath.Join(homedir, output))
	if err != nil {
		t.Fatal(err)
	}
}
