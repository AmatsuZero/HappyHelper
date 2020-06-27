package test

import (
	"HappyHelper"
	"HappyHelper/ehentai"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestCreateTable(t *testing.T) {
	gallery := ehentai.Gallery{}
	err := ehentai.DefaultSerializeManager.CreateTable(gallery.CreteTableQuery())
	assert.NoError(t, err)
	err = ehentai.DefaultSerializeManager.Close()
	assert.NoError(t, err)
}

func TestWriteData(t *testing.T) {
	testGallery, err := ehentai.RestoreGallery("http://www.foo.com/bar")
	assert.NoError(t, err)
	_, err = testGallery.Serialize()
	assert.NoError(t, err)
	db, err := ehentai.DefaultSerializeManager.GetDB()
	assert.NoError(t, err)
	err = db.Close()
	assert.NoError(t, err)
}

func TestUpdateData(t *testing.T) {
	testGallery, err := ehentai.RestoreGallery("http://www.foo.com/bar")
	assert.NoError(t, err)
	testGallery.Links = []string{"http://www.foo.com/bar/4", "http://www.foo.com/bar/5", "http://www.foo.com/bar/6"}
	testGallery.BookName = "H Book"
	id, err := testGallery.Serialize()
	assert.NoError(t, err)
	t.Log(id)
}

func TestRestoreData(t *testing.T) {
	lhs := &ehentai.Gallery{
		Src:      "http://www.foo.com/bar",
		Links:    []string{"http://www.foo.com/bar/4", "http://www.foo.com/bar/5", "http://www.foo.com/bar/6"},
		BookName: "H Book",
	}
	_, err := lhs.Serialize()
	assert.NoError(t, err)
	rhs, err := ehentai.RestoreGallery(lhs.Src)
	assert.NoError(t, err)
	assert.Equal(t, lhs, rhs)
}

func TestDeleteData(t *testing.T) {
	src := "http://www.foo.com/bar"
	_, err := ehentai.DefaultSerializeManager.Delete("gallery", map[string]interface{}{
		"src": "http://www.foo.com/bar",
	})
	assert.NoError(t, err)
	result, err := ehentai.RestoreGallery(src)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestDeleteGallery(t *testing.T) {
	t.Log(os.Getenv("https_proxy"))
	parser := ehentai.NewEHParser()
	snapShot := HappyHelper.NewSnapShot(parser)
	cancel, err := snapShot.Parse("https://e-hentai.org/g/835164/effdba09ab/")
	if err != nil {
		t.Fatal(err)
	}
	<-cancel
	t.Log(os.TempDir())

	g := parser.DownloadItem.(*ehentai.Gallery)
	assert.NoError(t, g.Delete())
}
