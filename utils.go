package HappyHelper

import (
	"archive/zip"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const OverLimitError = "Over Limit"
const OverLimitErrorPage = "https://ehgt.org/g/509.gif"

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
	defer func() {
		_ = fz.Close()
	}()
	w := zip.NewWriter(fz)
	defer func() {
		_ = w.Close()
	}()
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
		defer func() {
			_ = fSrc.Close()
		}()
		_, err = io.Copy(fDest, fSrc)
		if err != nil {
			return err
		}
		return nil
	})
	return
}

func CleanAllTmpDirs(links map[string][]string) {
	for tmpDir := range links {
		_, err := os.Stat(tmpDir)
		if os.IsNotExist(err) { // 已经不存在了，略过
			continue
		}
		if err := os.RemoveAll(tmpDir); err != nil { // 删除临时文件夹
			fmt.Printf("删除临时文件夹失败 %v\n", err)
		}
	}
}

func ZipFiles(path string, links map[string][]string) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) { // 检查输出文件夹是否存在
		err = os.MkdirAll(path, os.ModePerm)
	}
	if err != nil {
		return err
	}
	// 标记已经压缩过的
	flags := make(map[string]bool)
	for tmpDir, _ := range links {
		output := filepath.Join(path, filepath.Base(tmpDir)) + ".zip"
		if _, ok := flags[output]; ok {
			continue
		}
		fmt.Printf("正在创建压缩包: %v\n", output)
		err := ZipDir(output, tmpDir) // 创建压缩包
		if err != nil {
			fmt.Printf("创建压缩包失败: %v\n", output)
		}
		flags[output] = true
	}
	return nil
}

type Paths []string

func NewPaths(u *url.URL) Paths {
	return strings.Split(u.Path, "/")
}

func (paths *Paths) Append(path string) {
	*paths = append(*paths, path)
	return
}

func (paths Paths) LastComponent() string {
	if len(paths) == 0 {
		return ""
	}
	return paths[len(paths)-1]
}

func (paths *Paths) RemoveLast() (p string) {
	if len(*paths) == 0 {
		return ""
	}
	p, *paths = (*paths)[len(*paths)-1], (*paths)[:len(*paths)-1]
	return
}

func (paths *Paths) Del(path string) {
	index := -1
	for i, e := range *paths {
		if e == path {
			index = i
			break
		}
	}
	if index == -1 {
		return
	}
	*paths = append((*paths)[:index], (*paths)[index:]...)
	return
}

func (paths Paths) Encode() string {
	result := make([]string, len(paths))
	for i, path := range paths {
		result[i] = url.QueryEscape(path)
	}
	return strings.Join(paths, "/")
}
