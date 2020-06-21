package HappyHelper

import (
	"archive/zip"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const DefaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.149 Safari/537.36 OPR/67.0.3575.115"

/// 创建临时文件夹
func MKTmpDirIfNotExist(title string) (string, error) {
	// 创建临时文件夹
	dir := filepath.Join(os.TempDir(), title)
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, os.ModePerm)
	}
	return dir, err
}

func GetFileName(url *url.URL) string {
	path := url.Path
	segments := strings.Split(path, "/")
	return segments[len(segments)-1]
}

func PutFile(name, dst string, body io.Reader) (string, error) {
	f := filepath.Join(dst, name)
	file, err := os.Create(f)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(file, body)
	defer func() {
		_ = file.Close()
	}()
	return f, err
}

func ZipDir(dst, dir string) (err error) {
	fz, err := os.Create(dst)
	if err != nil {
		return
	}
	defer fz.Close()
	w := zip.NewWriter(fz)
	defer w.Close()
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() || err != nil {
			return err
		}
		fDest, err := w.Create(path[len(dir)+1:])
		if err != nil {
			return err
		}
		fSrc, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fSrc.Close()
		_, err = io.Copy(fDest, fSrc)
		if err != nil {
			return err
		}
		return nil
	})
	return
}
