package test

import (
	"HappyHelper"
	"HappyHelper/ehentai"
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

func TestDownloadSinglePage(t *testing.T) {
	t.Log(os.Getenv("https_proxy"))
	parser := ehentai.NewEHParser()
	snapShot := HappyHelper.NewSnapShot(parser)
	cancel := snapShot.Parse("https://e-hentai.org/g/835164/effdba09ab/")
	<-cancel
	t.Log(os.TempDir())
	myself, err := user.Current()
	if err != nil {
		t.Fail()
	}
	if myself == nil {
		return
	}
	homedir := myself.HomeDir
	output := filepath.Join("Desktop", "output")
	cancel = snapShot.Download(filepath.Join(homedir, output))
	if err != nil {
		t.Fatal(err)
	}
	<-cancel
}

func TestParseGrid(t *testing.T) {
	t.Log(os.Getenv("https_proxy"))
	parser := ehentai.NewEHParser()
	snapShot := HappyHelper.NewSnapShot(parser)
	cancel := snapShot.Parse("https://e-hentai.org/tag/artist:urakan")
	<-cancel
	//t.Log(os.TempDir())
	//myself, err := user.Current()
	//if err != nil {
	//	t.Fail()
	//}
	//homedir := myself.HomeDir
	//output := filepath.Join("Desktop", "output")
	//err = snapShot.Download(filepath.Join(homedir, output))
	//if err != nil {
	//	t.Fatal(err)
	//}
}
