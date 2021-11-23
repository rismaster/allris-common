package application

import (
	"cloud.google.com/go/datastore"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"context"
	"fmt"
	"github.com/algolia/algoliasearch-client-go/v3/algolia/search"
	"github.com/mailgun/mailgun-go/v4"
	allris_common "github.com/rismaster/allris-common"
	"github.com/rismaster/allris-common/common/slog"
	"github.com/rismaster/allris-common/downloader"
)

type AppContext struct {
	context context.Context

	storageClient   *storage.Client
	httpClient      *downloader.RetryClient
	publisher       *pubsub.Client
	datastoreClient *datastore.Client
	mailer          *mailgun.MailgunImpl

	searchClient *search.Client
	searchIndex  *search.Index

	Config    allris_common.Config
	TopicDone *pubsub.Topic
}

func (app *AppContext) Publisher() *pubsub.Client {

	if app.publisher == nil {
		client, err := pubsub.NewClient(app.context, app.Config.GetProjectId())
		if err != nil {
			slog.Fatal("error creating publisher %v", err)
		}
		app.publisher = client

		app.TopicDone, err = app.getOrCrerateTopic(app.Config.GetPublicSearchIndexDoneTopic())
		if err != nil {
			slog.Fatal(fmt.Sprintf("error init topic %s - %v", err, app.Config.GetPublicSearchIndexDoneTopic()), err)
		}
	}
	return app.publisher
}

func (app *AppContext) Mail() *mailgun.MailgunImpl {
	if app.mailer == nil {
		app.mailer = mailgun.NewMailgun(app.Config.GetMailDomain(), app.Config.GetMailApiString())
	}
	return app.mailer
}

func (app *AppContext) Db() *datastore.Client {
	if app.datastoreClient == nil {
		var err error
		app.datastoreClient, err = datastore.NewClient(app.context, app.Config.GetProjectId())
		if err != nil {
			slog.Fatal("error creating datastoreClient %v", err)
		}
	}
	return app.datastoreClient
}

func (app *AppContext) Store() *storage.Client {
	if app.storageClient == nil {
		var err error
		app.storageClient, err = storage.NewClient(app.context)
		if err != nil {
			slog.Fatal("error creating storageClient %v", err)
		}
	}
	return app.storageClient
}

func (app *AppContext) Http() *downloader.RetryClient {

	if app.httpClient == nil {

		app.httpClient = &downloader.RetryClient{
			Config:           app.Config,
			Timeout:          app.Config.GetHttpTimeout(),
			CallDelay:        app.Config.GetHttpCalldelay(),
			Versuche:         app.Config.GetHttpVersuche(),
			WithProxy:        app.Config.GetHttpWithproxy(),
			WartezeitOnRetry: app.Config.GetHttpWartezeitonretry(),
			ProxParser:       app.Config.GetProxyParser(),
		}
	}
	return app.httpClient
}

func (app *AppContext) Ctx() context.Context {
	return app.context
}

func (app *AppContext) Search() *search.Client {

	if app.searchClient == nil {
		app.searchClient = search.NewClient(app.Config.GetSearchAppId(), app.Config.GetSearchApiKey())
	}
	return app.searchClient
}

func (app *AppContext) SearchIndex() *search.Index {

	if app.searchIndex == nil {
		app.searchIndex = app.Search().InitIndex(app.Config.GetSearchIndex())
	}
	return app.searchIndex
}

func NewAppContextWithContext(ctx context.Context, conf allris_common.Config) *AppContext {

	appContext := new(AppContext)
	appContext.context = ctx
	appContext.Config = conf
	return appContext
}

func NewAppContext(conf allris_common.Config) *AppContext {

	appContext := new(AppContext)
	appContext.context = context.Background()
	appContext.Config = conf
	return appContext
}

func (appContext *AppContext) getOrCrerateTopic(topicName string) (*pubsub.Topic, error) {
	topic := appContext.publisher.Topic(topicName)
	exists, err := topic.Exists(appContext.Ctx())
	if err != nil {
		return nil, err
	}
	if !exists {
		topic, err = appContext.publisher.CreateTopic(appContext.Ctx(), topicName)
		if err != nil {
			return nil, err
		}
	}
	return topic, nil
}
