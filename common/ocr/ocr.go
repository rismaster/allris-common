package ocr

import (
	"encoding/json"
	"github.com/rismaster/allris-common/application"
	vision2 "google.golang.org/api/vision/v1"
	"io/ioutil"
)

type OcrGcsSource struct {
	Uri string
}

type OcrInputConfig struct {
	GcsSource OcrGcsSource
	MimeType  string
}

type OcrFullTextAnnotation struct {
	Text  string
	Pages []vision2.Page
}

type OcrContext struct {
	Uri        string
	PageNumber int
}

type OcsResponse struct {
	FullTextAnnotation OcrFullTextAnnotation `datastore:"annot,noindex"`
	Context            OcrContext            `datastore:"ctx,noindex"`
}

type OcrJsonoutput struct {
	InputConfig OcrInputConfig
	Responses   []OcsResponse
}

func readJson(content []byte) (*OcrJsonoutput, error) {

	var jsonOut OcrJsonoutput
	err := json.Unmarshal(content, &jsonOut)
	if err != nil {
		return nil, err
	}
	return &jsonOut, nil
}

func ReadOcrFromFile(appContext *application.AppContext, fileName string, bucketName string) (*OcrJsonoutput, error) {
	reader, err := appContext.Store().Bucket(bucketName).Object(fileName).NewReader(appContext.Ctx())
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	content, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return readJson(content)
}
