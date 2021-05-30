package downloader

import (
	"github.com/kennygrant/sanitize"
	"net/url"
	"time"
)

type RisRessource struct {
	Uri     *url.URL
	Created time.Time
	Folder  string
	Name    string
	Ending  string

	FormData *url.Values
}

func NewRisRessource(folder string, name string, ending string, created time.Time, uri *url.URL, formData *url.Values) *RisRessource {

	return &RisRessource{
		Folder:   folder,
		Name:     sanitize.Path(name),
		Ending:   ending,
		Created:  created,
		Uri:      uri,
		FormData: formData,
	}
}

func (r *RisRessource) GetFolder() string {
	return r.Folder
}

func (r *RisRessource) GetName() string {
	return r.Name
}

func (r *RisRessource) GetEnding() string {
	return r.Ending
}

func (r *RisRessource) GetCreated() time.Time {
	return r.Created
}

func (r *RisRessource) GetUrl() string {
	if r.Uri == nil {
		return ""
	}
	return r.Uri.String()
}

func (r *RisRessource) GetFormData() *url.Values {
	return r.FormData
}
