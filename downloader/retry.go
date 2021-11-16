package downloader

import (
	"bytes"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"
	allris_common "github.com/rismaster/allris-common"
	"github.com/rismaster/allris-common/common/slog"
	"golang.org/x/net/html/charset"
	"h12.io/socks"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type RetryClient struct {
	Config           allris_common.Config
	client           *http.Client
	WithProxy        bool
	Timeout          time.Duration
	Versuche         int
	durchlauf        int
	CallDelay        time.Duration
	WartezeitOnRetry time.Duration
	ProxParser       ProxParser
}

type ProxParser interface {
	Parse(body []byte) (*url.URL, error)
}

type ProxyUrl struct {
	Ip   string
	Port int
}

type stop struct {
	error
}

func (retryClient *RetryClient) Retry(f func(client *http.Client) error) error {

	if retryClient.client == nil {
		client, err := retryClient.getHttpClient()
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("error init httpclient %v", err))
		}
		retryClient.client = client
	}

	if err := retryClient.fun(f); err != nil {
		retryClient.client = nil

		slog.Warn("Error on retry (attempts left: %d): %v", retryClient.Versuche-retryClient.durchlauf, err)

		if s, ok := err.(stop); ok {
			return s.error
		}

		retryClient.durchlauf = retryClient.durchlauf + 1

		if retryClient.durchlauf < retryClient.Versuche {
			// Add some randomness to prevent creating a Thundering Herd
			jitter := time.Duration(rand.Int63n(int64(retryClient.WartezeitOnRetry)))
			retryClient.WartezeitOnRetry = retryClient.WartezeitOnRetry + jitter/6

			time.Sleep(retryClient.WartezeitOnRetry)
			return retryClient.Retry(f)
		}
		return err
	}

	return nil
}

func (retryClient *RetryClient) fun(f func(client *http.Client) error) error {

	cd := int64(retryClient.CallDelay)
	jitter := time.Duration(rand.Int63n(cd + 1))
	time.Sleep(retryClient.CallDelay + jitter/3)
	err := f(retryClient.client)
	if err == nil {
		retryClient.durchlauf = 0
	}
	return err
}

func (retryClient *RetryClient) getProxy() (*url.URL, error) {

	req, _ := http.NewRequest("GET", retryClient.Config.GetProxyUrl(), nil)
	req.Header.Add(retryClient.Config.GetProxySecretHeaderKey(), retryClient.Config.GetProxySecret())
	req.Header.Add(retryClient.Config.GetProxyHostHeaderKey(), retryClient.Config.GetProxyHost())

	res, _ := http.DefaultClient.Do(req)

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	err = res.Body.Close()
	if err != nil {
		return nil, err
	}

	parser := retryClient.ProxParser
	url, err := parser.Parse(body)

	return url, err
}

func (retryClient *RetryClient) getHttpClient() (*http.Client, error) {

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		//Transport: tr,
		Timeout: retryClient.Timeout,
		Jar:     jar,
	}

	if retryClient.WithProxy {
		proxyUrl, err := retryClient.getProxy()
		if err != nil {
			return nil, err
		}

		var tr = &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		}

		if !strings.HasPrefix(proxyUrl.Scheme, "http") {
			dialSocksProxy := socks.Dial(proxyUrl.String())
			tr = &http.Transport{Dial: dialSocksProxy}
		}
		httpClient.Transport = tr
	}

	return httpClient, nil
}

func (retryClient *RetryClient) FetchFromInternetWithPost(ris *RisRessource) (file *Download, e error) {

	var name string
	var statusCode int
	var body []byte
	var contentType string

	e = retryClient.Retry(func(client *http.Client) error {

		encodedUrl := ris.FormData.Encode()
		slog.Info("send post request: %s?%s", ris.GetUrl(), encodedUrl)
		r, _ := http.NewRequest(http.MethodPost, ris.GetUrl(), strings.NewReader(encodedUrl)) // URL-encoded payload
		r.Header.Add("content-Type", "application/x-www-form-urlencoded")
		r.Header.Add("content-Length", strconv.Itoa(len(encodedUrl)))

		resp, err := client.Do(r)
		if err != nil {
			return err
		}

		contentType = resp.Header.Get("content-type")
		name = path.Base(resp.Request.URL.String())

		statusCode = resp.StatusCode
		if statusCode == 404 {
			slog.Warn(fmt.Sprintf("error fetching: %s | %d", ris.GetUrl(), statusCode))
		} else if statusCode != 200 {
			return errors.New(fmt.Sprintf("error fetching: %s | %d", ris.GetUrl(), statusCode))
		} else if strings.HasPrefix(contentType, "text/html") {
			body, contentType, err = retryClient.readHtmlBodyAndContentType(contentType, resp)
			if err != nil {
				return err
			}

		} else {
			body, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}

			err = resp.Body.Close()
			if err != nil {
				return err
			}

			if len(body) <= 0 {
				return errors.New(fmt.Sprintf("error empty body: %s", ris.GetUrl()))
			}
		}
		return nil
	})

	if e != nil {
		return nil, e
	}

	return NewDownload(name, contentType, body, statusCode), nil
}

func (retryClient *RetryClient) FetchFromInternetWithGet(uri string) (file *Download, respErr error) {

	var name string
	var body []byte
	var contentType string
	var statusCode int

	respErr = retryClient.Retry(func(client *http.Client) error {

		req, err := http.NewRequest(http.MethodGet, uri, nil)
		if err != nil {
			return err
		}

		slog.Debug("send request: %s", uri)
		resp, err := client.Do(req)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("error fetching %s: %+v", uri, err))
		}

		statusCode = resp.StatusCode

		xpageHeader := resp.Header.Get("X-Page")
		if xpageHeader == "noauth.asp" {
			return stop{errors.New(fmt.Sprintf("error fetching X-Page=noauth.asp - no retry : %s | %d", uri, statusCode))}
		}

		if statusCode != 200 {
			return errors.New(fmt.Sprintf("error fetching: %s | %d", uri, statusCode))
		}

		var headerContentType = strings.ReplaceAll(
			strings.ToLower(resp.Header.Get("content-Type")), " ", "")

		if strings.HasPrefix(headerContentType, "text/html") {
			body, contentType, err = retryClient.readHtmlBodyAndContentType(headerContentType, resp)
			if err != nil {
				return err
			}

		} else {
			contentType = headerContentType
			body, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
		}

		err = resp.Body.Close()
		if err != nil {
			return err
		}

		name = path.Base(resp.Request.URL.String())

		return nil
	})
	if respErr != nil {
		return nil, respErr
	}

	return NewDownload(name, contentType, body, statusCode), nil
}

func (retryClient *RetryClient) readHtmlBodyAndContentType(headerContentType string, resp *http.Response) ([]byte, string, error) {
	var readerCharset string
	var resultContentType string
	var result []byte
	if headerContentType == "text/html" || headerContentType == "text/html;charset=iso-8859-1" {
		//bis html 4.x default charset
		readerCharset = "latin"
		headerContentType = "text/html;charset=iso-8859-1"
	} else {
		splitedCt := strings.Split(headerContentType, ";")
		if len(splitedCt) == 2 {
			readerCharset = splitedCt[1]
		} else {
			readerCharset = "utf-8"
		}
	}
	reader, err := charset.NewReader(resp.Body, readerCharset)
	if err != nil {
		return nil, "", err
	}
	result, err = ioutil.ReadAll(reader)
	if err != nil {
		return nil, "", err
	}

	resultContentType, err = retryClient.GetContentType(result)
	if err != nil {
		slog.Warn("missing contentType in body, use header %s", headerContentType)
		resultContentType = headerContentType
	}

	if resultContentType != headerContentType {
		if strings.HasSuffix(resultContentType, "utf-8") && headerContentType == "text/html;charset=iso-8859-1" {
			result = iso8859ToUtf8(result)
		} else {
			slog.Warn("different contentType (header %s, body %s) but i do not know how to fix it", headerContentType, resultContentType)
		}
	}
	return result, "text/html;charset=utf-8", nil
}

func (retryClient *RetryClient) GetContentType(bodyData []byte) (contentType string, err error) {

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bodyData))
	if err != nil {
		return "", errors.New(fmt.Sprintf("error create dom to get contenttype %v", err))
	}
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		if name, _ := s.Attr("http-equiv"); strings.EqualFold(name, "content-type") {
			ct, exist := s.Attr("content")
			if exist {
				contentType = strings.ReplaceAll(strings.ToLower(ct), " ", "")
			}
		}
	})
	if contentType == "" {
		return "", errors.New(fmt.Sprintf("contenttype is empty"))
	}
	return contentType, nil
}

func iso8859ToUtf8(iso88591Buf []byte) []byte {
	buf := make([]rune, len(iso88591Buf))
	for i, b := range iso88591Buf {
		buf[i] = rune(b)
	}
	return []byte(string(buf))
}
