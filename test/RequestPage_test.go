package test

import (
	"HappyHelper"
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

func TestDownloadSinglePage(t *testing.T) {
	t.Log(os.Getenv("https_proxy"))
	parser := HappyHelper.NewEHParser()
	snapShot := HappyHelper.NewSnapShot(parser)
	err := snapShot.Parse("https://e-hentai.org/g/835164/effdba09ab/")
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

func TestParseGrid(t *testing.T) {
	t.Log(os.Getenv("https_proxy"))
	parser := HappyHelper.NewEHParser()
	snapShot := HappyHelper.NewSnapShot(parser)
	err := snapShot.Parse("https://e-hentai.org")
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
