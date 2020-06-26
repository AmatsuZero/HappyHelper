package ehentai

import (
	"HappyHelper"
	"context"
	"database/sql"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	_ "github.com/mattn/go-sqlite3"
	"github.com/reactivex/rxgo/v2"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

func requestSinglePage(src string, client *http.Client) (*goquery.Document, error) {
	u, err := url.Parse(src)
	if err != nil {
		return nil, err
	}
	resp, err := client.Get(src)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	d, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}
	d.Url = u
	return d, err
}

func wrappedRequestSinglePage(src string, client *http.Client) rxgo.Observable {
	return rxgo.Create([]rxgo.Producer{func(ctx context.Context, next chan<- rxgo.Item) {
		doc, err := requestSinglePage(src, client)
		next <- rxgo.Item{
			V: doc,
			E: err,
		}
	}})
}

func retryStrategy(err error) bool {
	if err.Error() == HappyHelper.OverLimitError { // TODO：如果超出下载配额，可能下载链接都是509错误页，待解决
		return false
	} else if err.Error() == "no doc" {
		return false
	} else if e, ok := err.(*url.Error); ok {
		switch e.Err.Error() {
		case "Too many open connections": // TODO: 待解决
			return true
		case "EOF": // 可能超过了服务器的允许的连接数
			return false
		}
	}
	return true
}

type SerializeManager struct {
	db *sql.DB
}

var DefaultSerializeManager *SerializeManager

func init() {
	DefaultSerializeManager = &SerializeManager{}
}

func (mgr *SerializeManager) GetDB() (*sql.DB, error) {
	if mgr.db != nil {
		return mgr.db, nil
	}
	current, err := user.Current()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(current.HomeDir, ".happy_helper_cache")
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		err = HappyHelper.MKDirIfNotExist(path)
	}
	if err != nil {
		return nil, err
	}
	path = filepath.Join(path, "ehentai.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	mgr.db = db
	return mgr.db, nil
}

func (mgr *SerializeManager) openDB() error {
	current, err := user.Current()
	if err != nil {
		return err
	}
	path := filepath.Join(current.HomeDir, ".happy_helper_cache")
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		err = HappyHelper.MKDirIfNotExist(path)
	}
	if err != nil {
		return err
	}
	path = filepath.Join(path, "ehentai.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return err
	}
	mgr.db = db
	return nil
}

func (mgr *SerializeManager) CreateTable(query string) error {
	db, err := mgr.GetDB()
	if err != nil {
		return err
	}
	stat, err := db.Prepare(query)
	if err != nil {
		return err
	}
	_, err = stat.Exec()
	return err
}

func (mgr *SerializeManager) Insert(tableName string, args map[string]interface{}) (int64, error) {
	db, err := mgr.GetDB()
	if err != nil {
		return -1, err
	}
	query := fmt.Sprintf("INSERT INTO %v(", tableName)
	keys := make([]string, 0, len(args))
	values := make([]interface{}, 0, len(args))
	questionMarks := make([]string, 0, len(args))
	for k, v := range args {
		keys = append(keys, k)
		values = append(values, v)
		questionMarks = append(questionMarks, "?")
	}
	query += strings.Join(keys, ",")
	query += fmt.Sprintf(") values(%v)", strings.Join(questionMarks, ","))
	stmt, err := db.Prepare(query)
	if err != nil {
		return -1, err
	}
	res, err := stmt.Exec(values...)
	if err != nil {
		return -1, err
	}
	return res.LastInsertId()
}

func (mgr *SerializeManager) Update(tableName string, update, condition map[string]interface{}) (int64, error) {
	db, err := mgr.GetDB()
	if err != nil {
		return 0, err
	}
	query := fmt.Sprintf("UPDATE %v set ", tableName)
	values := make([]interface{}, 0, len(update))
	keys := make([]string, 0, len(update))
	for k, v := range update {
		keys = append(keys, fmt.Sprintf("%v=?", k))
		values = append(values, v)
	}
	query += strings.Join(keys, ",")
	query = query + " where "

	keys = make([]string, 0, len(condition))
	for k, v := range condition {
		keys = append(keys, fmt.Sprintf("%v=?", k))
		values = append(values, v)
	}
	query += strings.Join(keys, ",")
	stmt, err := db.Prepare(query)
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(values...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (mgr *SerializeManager) GetRows(tableName string, condition map[string]interface{}) (*sql.Rows, error) {
	db, err := mgr.GetDB()
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf("SELECT * from %v where ", tableName)
	keys := make([]string, 0, len(condition))
	values := make([]interface{}, 0, len(condition))
	for k, v := range condition {
		keys = append(keys, fmt.Sprintf("%v=?", k))
		values = append(values, v)
	}
	query += strings.Join(keys, ",")
	return db.Query(query, values...)
}

func (mgr *SerializeManager) Delete(table string, condition map[string]interface{}) (int64, error) {
	db, err := mgr.GetDB()
	if err != nil {
		return 0, err
	}
	query := fmt.Sprintf("DELETE from %v where ", table)
	keys := make([]string, 0, len(condition))
	values := make([]interface{}, 0, len(condition))
	for k, v := range condition {
		keys = append(keys, fmt.Sprintf("%v=?", k))
		values = append(values, v)
	}
	query += strings.Join(keys, ",")
	stmt, err := db.Prepare(query)
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(values...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (mgr *SerializeManager) Close() error {
	if mgr == nil || mgr.db == nil {
		return nil
	}
	return mgr.db.Close()
}
