package downloader

type Download struct {
	name        string
	contentType string
	content     []byte
	statusCode  int
}

func NewDownload(name string, contentType string, content []byte, statusCode int) *Download {
	return &Download{
		name:        name,
		contentType: contentType,
		content:     content,
		statusCode:  statusCode,
	}
}

func (r *Download) GetName() string {
	return r.name
}

func (r *Download) GetContentType() string {
	return r.contentType
}

func (r *Download) GetContent() []byte {
	return r.content
}
