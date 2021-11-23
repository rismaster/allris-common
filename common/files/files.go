package files

import (
	"bytes"
	"cloud.google.com/go/storage"
	"compress/gzip"
	"fmt"
	"github.com/kennygrant/sanitize"
	"github.com/pkg/errors"
	"github.com/rismaster/allris-common/application"
	"github.com/rismaster/allris-common/common/slog"
	"github.com/rismaster/allris-common/downloader"
	"google.golang.org/api/iterator"
	"io/ioutil"
	"path"
	"strings"
	"time"
)

const HttpGet = "GET"
const HttpPost = "POST"

type File struct {
	app         *application.AppContext
	folder      string    //folder in storage
	name        string    //filename with extension
	contentType string    //contentType (html or pdf supported)
	updated     time.Time //last time stored file updated
	risTime     time.Time //time the corresponding ressource in ris was created
	fetchedAt   time.Time
	hash        string //hash of the content created before saved in store
	content     []byte //the data of the stored file if loaded

	loadedFromStore    bool
	docInfoAlreadyRead bool //the properties of the file were already loaded from store
	existInStore       bool //true if the file exist in store
}

func (file *File) GetReader() *bytes.Reader {
	return bytes.NewReader(file.content)
}

func (file *File) GetName() string {
	return file.name
}

func (file *File) GetPath() string {
	return file.folder + file.name
}

func (file *File) GetFolder() string {
	return file.folder
}

func (file *File) GetNameWithoutExtension() string {
	basFileName := path.Base(file.GetName())
	extension := path.Ext(basFileName)
	return basFileName[:len(basFileName)-len(extension)]
}

func (file *File) GetExtension() string {
	return path.Ext(file.GetName())
}

func NewFileFromStore(app *application.AppContext, folder string, name string) *File {

	return &File{
		app:                app,
		folder:             folder,
		name:               name,
		docInfoAlreadyRead: false,
		existInStore:       false,
	}
}

// NewFile create clean new file without data from store
func NewFile(app *application.AppContext, webressource *downloader.RisRessource) *File {

	return &File{
		app:                app,
		folder:             webressource.GetFolder(),
		name:               webressource.GetName() + webressource.GetEnding(),
		docInfoAlreadyRead: false,
		existInStore:       false,
		risTime:            webressource.GetCreated(),
	}
}

// NewFileCopy full copy a file from another
func NewFileCopy(file *File) *File {

	return &File{
		app:                file.app,
		folder:             file.folder,
		name:               file.name,
		hash:               file.hash,
		contentType:        file.contentType,
		updated:            file.updated,
		risTime:            file.risTime,
		docInfoAlreadyRead: file.docInfoAlreadyRead,
		existInStore:       file.existInStore,
		content:            file.content,
		fetchedAt:          file.fetchedAt,
	}
}

// NewFileFromAttrs create a file from attributes of stored object
func NewFileFromAttrs(app *application.AppContext, attrs *storage.ObjectAttrs) (*File, error) {

	folder, name := path.Split(attrs.Name)

	fetchedAt, err := time.Parse(time.RFC3339, attrs.Metadata["fetchedAt"])
	if err != nil {
		return nil, err
	}

	return &File{
		app:                app,
		folder:             folder,
		name:               name,
		hash:               attrs.Metadata["hash"],
		contentType:        attrs.ContentType,
		updated:            attrs.Updated,
		risTime:            attrs.CustomTime,
		docInfoAlreadyRead: true,
		existInStore:       true,
		fetchedAt:          fetchedAt,
	}, nil
}

// ReadDocumentInfo load attributes from storage into file (only first time called it will get the attributes)
func (file *File) ReadDocumentInfo(bucket string) error {

	if !file.docInfoAlreadyRead {
		file.docInfoAlreadyRead = true

		attrs, err := file.app.Store().Bucket(bucket).Object(file.GetPath()).Attrs(file.app.Ctx())
		if err == storage.ErrObjectNotExist {
			return nil
		}
		if err != nil {
			return err
		}

		fetchedAt, err := time.Parse(time.RFC3339, attrs.Metadata["fetchedAt"])
		if err != nil {
			return err
		}

		file.existInStore = true
		file.hash = attrs.Metadata["hash"]
		file.updated = attrs.Updated
		file.contentType = attrs.ContentType
		file.risTime = attrs.CustomTime
		file.fetchedAt = fetchedAt
	}
	return nil
}

// createObjectAttrs create attributes from file
func (file *File) createObjectAttrs() (attrs *storage.ObjectAttrs, err error) {
	if file.hash == "" {
		return nil, errors.New(fmt.Sprintf("hash was not set for file %s", file.name))
	}
	if file.contentType == "" {
		return nil, errors.New(fmt.Sprintf("contentType was not set for file %s", file.name))
	}

	props := map[string]string{"hash": file.hash, "fetchedAt": file.fetchedAt.Format(time.RFC3339)}

	if file.existInStore {
		props["ChangedBy"] = "Update"
	} else {
		props["ChangedBy"] = "Create"
	}

	attrs = &storage.ObjectAttrs{
		Name:            file.GetPath(),
		ContentLanguage: "de",
		ContentType:     file.contentType,
		CustomTime:      file.risTime,
		Metadata:        props,
	}

	return attrs, nil
}

// Fetch fetch a file, first look for a file in storage if the file is new (app.MinAgeBeforeDownload)
// use this, if older load from internet
func (file *File) Fetch(httpMethod string, webRessource *downloader.RisRessource, expectedMimeType string, force bool) (fresh bool, err error) {

	//load file in store
	oldFile := NewFileCopy(file)
	err = oldFile.ReadDocumentInfo(file.app.Config.GetBucketFetched())
	if err != nil && err != storage.ErrObjectNotExist {
		return false, errors.Wrap(err, fmt.Sprintf("error reading old vorlage %s", oldFile.name))
	}

	tooNew := time.Now().Before(oldFile.updated.Add(file.app.Config.GetMinAgeBeforeDownload()))
	useStoredObj := !force && oldFile.existInStore && (!webRessource.Redownload || tooNew)

	if useStoredObj {
		fresh = false
		file.contentType = oldFile.contentType
		file.fetchedAt = oldFile.fetchedAt
		file.existInStore = true
		file.hash = oldFile.hash
		file.updated = oldFile.updated
		file.risTime = oldFile.risTime

		slog.Debug("Read From Store: %s", file.GetPath())
		err = oldFile.ReadDocument(file.app.Config.GetBucketFetched())
		if err != nil {
			return false, errors.Wrap(err, fmt.Sprintf("error getting file content from store %s is %s", webRessource.GetUrl(), oldFile.name))
		}

		file.content = oldFile.content
		file.loadedFromStore = true

	} else {
		slog.Info("%s: %s (%s)", httpMethod, webRessource.GetName(), webRessource.GetUrl())

		var download *downloader.Download
		if httpMethod == HttpGet {
			download, err = file.app.Http().FetchFromInternetWithGet(webRessource.GetUrl())
			if err != nil {
				return false, errors.Wrap(err, fmt.Sprintf("error fetching file %s, Error: %v", webRessource.GetUrl(), err))
			}

		} else {
			download, err = file.app.Http().FetchFromInternetWithPost(webRessource)
			if err != nil {
				return false, errors.Wrap(err, fmt.Sprintf("error fetching file %s, Error: %v", webRessource.GetUrl(), err))
			}
		}

		if expectedMimeType != "*" && !strings.HasPrefix(download.GetContentType(), expectedMimeType) {
			return false, errors.New(fmt.Sprintf("content is not %s on page %s is %s", expectedMimeType, webRessource.GetUrl(), download.GetContentType()))
		}

		file.fetchedAt = time.Now()
		file.contentType = download.GetContentType()
		file.content = download.GetContent()
		file.loadedFromStore = false
		fresh = true
	}

	return fresh, nil
}

func (file *File) backupAndUpdateFile() error {
	return file.moveToBackup(false)
}

// WriteIfMoreActualAndDifferent write file to store if the file not exist there or the given hash is different
func (file *File) WriteIfMoreActualAndDifferent(newHash string) (err error) {

	err = file.ReadDocumentInfo(file.app.Config.GetBucketFetched())
	if err != nil {
		return err
	}

	if file.existInStore {

		if file.hash == newHash {
			slog.Debug("Same Hash for File %s: %s", file.GetPath(), file.hash)

			if !file.loadedFromStore {

				err = file.touch(file.app.Config.GetBucketFetched())
				if err != nil {
					return errors.Wrap(err, fmt.Sprintf("error touching file %s", file.GetPath()))
				}
			}
			return nil
		} else {
			slog.Debug("Hash different from Download for existing File %s: %s", file.GetPath(), file.hash)
		}

		err = file.backupAndUpdateFile()
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("error moving file '%s' to backup", file.GetPath()))
		}
		slog.Info("backup successfully, Update File: %s (%s)", file.GetPath(), file.hash)
	} else {
		slog.Info("Create File: %s", file.GetPath())
	}

	file.hash = newHash
	err = file.writeDocument(file.app.Config.GetBucketFetched())
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("error writing new file %s", file.GetPath()))
	}

	return err
}

func (file *File) GetContent() []byte {
	return file.content
}

func (file *File) GetContentType() string {
	return file.contentType
}

// moveToBackup move a stored file to the backup storage
func (file *File) moveToBackup(deleteOriginal bool) error {

	err := file.ReadDocument(file.app.Config.GetBucketFetched())
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("error deleting file %s", file.name))
	}

	newFile := NewFileCopy(file)
	newFile.name = sanitize.Path(
		fmt.Sprintf("%s_%s%s",
			file.GetNameWithoutExtension(),
			file.updated.Format("2006-01-02-15-04-05"),
			file.GetExtension()))

	err = newFile.writeDocument(file.app.Config.GetBucketBackup())
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("error writing file to backup %s", file.name))
	}

	if deleteOriginal {
		err = file.DeleteDocument(file.app.Config.GetBucketFetched())
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("error deleting file %s", file.name))
		}
	}

	return nil
}

func (file *File) backupAndDeleteFile() error {
	return file.moveToBackup(true)
}

// DeleteFilesIfNotInAndAfter delete files with given prefix if not in foundPrefixes
// delete children if stored file was deleted
func DeleteFilesIfNotInAndAfter(
	app *application.AppContext, prefix string, foundFilePathes map[string]bool, childFolders []string, minTime time.Time) error {

	it := app.Store().Bucket(app.Config.GetBucketFetched()).Objects(app.Ctx(), &storage.Query{
		Prefix: prefix,
	})

	var toDelete []*File

	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errors.Wrap(err, "error iterating file results")
		}

		if attrs.CustomTime.After(minTime) {

			_, exist := foundFilePathes[attrs.Name]
			if !exist {
				slog.Info("DELETE File '%s' not existing in RIS and backup it", attrs.Name)

				newFile, errAttr := NewFileFromAttrs(app, attrs)
				if errAttr != nil {
					return errors.Wrap(errAttr, fmt.Sprintf("could not create file from attrs %s", attrs.Name))
				}
				toDelete = append(toDelete, newFile)
			}
		}
	}

	for _, f := range toDelete {

		err := f.backupAndDeleteFile()
		if err != nil {
			slog.Error("error deleting file: %s %v", f.name, err)
			continue
		}

		for _, childFolder := range childFolders {
			childPrefixWithPath := childFolder + f.GetNameWithoutExtension()
			children, err := ListFiles(app, childPrefixWithPath)
			if err != nil {
				slog.Error("error reading files: %s %v", childPrefixWithPath, err)
				continue
			}
			for _, child := range children {
				slog.Info("DELETE Child from '%s' and backup it %s", f.name, child.GetPath())
				err = child.moveToBackup(true)
				if err != nil {
					slog.Error("error deleting file: %s %v", child.GetPath(), err)
					continue
				}
			}
		}
	}
	return nil
}

// ListFiles list Fileinfos from storage
func ListFiles(app *application.AppContext, prefix string) (result []*File, err error) {

	it := app.Store().Bucket(app.Config.GetBucketFetched()).Objects(app.Ctx(), &storage.Query{
		Prefix: prefix,
	})

	for {
		attrs, errIt := it.Next()
		if errIt == iterator.Done {
			break
		}
		if errIt != nil {
			return nil, errors.Wrap(errIt, fmt.Sprintf("error iterating file results %s", attrs.Name))
		}

		newFile, errAttr := NewFileFromAttrs(app, attrs)
		if errAttr != nil {
			return nil, errors.Wrap(errAttr, fmt.Sprintf("could not create file from attrs %s", attrs.Name))
		}
		result = append(result, newFile)
	}

	return result, nil
}

// ReadDocument read content from storage
func (file *File) ReadDocument(bucket string) error {

	reader, err := file.app.Store().Bucket(bucket).Object(file.GetPath()).NewReader(file.app.Ctx())
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}

	file.content = body

	err = reader.Close()
	if err != nil {
		return err
	}

	return nil
}

func (file *File) DeleteDocument(bucket string) error {
	return file.app.Store().Bucket(bucket).Object(file.GetPath()).Delete(file.app.Ctx())
}

func (file *File) touch(bucket string) error {
	slog.Info("Touch file: %s", file.GetPath())
	_, err := file.app.Store().Bucket(bucket).
		Object(file.GetPath()).
		Update(file.app.Ctx(), storage.ObjectAttrsToUpdate{})
	return err
}

func (file *File) writeDocument(bucket string) error {

	if len(file.content) == 0 {
		return errors.New(fmt.Sprintf("content length is 0 for file %s", file.name))
	}

	attrs, err := file.createObjectAttrs()
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("error reading attrs for file %s", file.name))
	}

	wc := file.app.Store().Bucket(bucket).
		Object(file.GetPath()).
		NewWriter(file.app.Ctx())

	wc.ObjectAttrs = *attrs

	wc.ObjectAttrs.ContentEncoding = "gzip"
	w := gzip.NewWriter(wc)

	_, err = w.Write(file.content)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	err = wc.Close()
	if err != nil {
		return err
	}

	return nil
}
