package test

import (
	"HappyHelper"
	"HappyHelper/ehentai"
	"os"
	"testing"
)

func TestDownloadSinglePage(t *testing.T) {
	t.Log(os.Getenv("https_proxy"))
	parser := ehentai.NewEHParser()
	snapShot := HappyHelper.NewSnapShot(parser)
	cancel, err := snapShot.Parse("https://e-hentai.org/g/835164/effdba09ab/")
	if err != nil {
		t.Fatal(err)
	}
	<-cancel
	t.Log(os.TempDir())
	//myself, err := user.Current()
	//if err != nil {
	//	t.Fail()
	//}
	//if myself == nil {
	//	return
	//}
	//homedir := myself.HomeDir
	//output := filepath.Join("Desktop", "output")
	//cancel = snapShot.Download(filepath.Join(homedir, output))
	//if err != nil {
	//	t.Fatal(err)
	//}
	<-cancel
}

func TestParseGrid(t *testing.T) {
	t.Log(os.Getenv("https_proxy"))
	parser := ehentai.NewEHParser()
	snapShot := HappyHelper.NewSnapShot(parser)
	cancel, err := snapShot.Parse("https://e-hentai.org/tag/artist:urakan")
	if err != nil {
		t.Fatal(err)
	}
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
