package application

import (
	"cloud.google.com/go/datastore"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"context"
	"github.com/pkg/errors"
	allris_common "github.com/rismaster/allris-common"
	"github.com/rismaster/allris-common/downloader"
	"log"
)

type AppContext struct {
	storageClient   *storage.Client
	context         context.Context
	httpClient      *downloader.RetryClient
	publisher       *pubsub.Client
	datastoreClient *datastore.Client
	Config          allris_common.Config
}

func (app *AppContext) Publisher() *pubsub.Client {
	return app.publisher
}

func (app *AppContext) Db() *datastore.Client {
	return app.datastoreClient
}

func (app *AppContext) Store() *storage.Client {
	return app.storageClient
}

func (app *AppContext) Http() *downloader.RetryClient {
	return app.httpClient
}

func (app *AppContext) Ctx() context.Context {
	return app.context
}

func NewAppContextWithContext(ctx context.Context) (*AppContext, error) {

	appContext := new(AppContext)
	appContext.context = ctx
	err := initAppContext(appContext)
	return appContext, err
}

func NewAppContext() (*AppContext, error) {

	appContext := new(AppContext)
	appContext.context = context.Background()
	err := initAppContext(appContext)
	return appContext, err
}

func initAppContext(appContext *AppContext) error {

	if appContext.storageClient == nil {
		var err error
		appContext.storageClient, err = storage.NewClient(appContext.context)
		if err != nil {
			return errors.Wrap(err, "error creating storageClient")
		}
	}

	if appContext.datastoreClient == nil {
		var err error
		appContext.datastoreClient, err = datastore.NewClient(appContext.context, appContext.Config.GetProjectId())
		if err != nil {
			log.Panicf("error init datastore service %v", err)
		}
	}

	if appContext.httpClient == nil {

		appContext.httpClient = &downloader.RetryClient{
			Config:           appContext.Config,
			Timeout:          appContext.Config.GetHttpTimeout(),
			CallDelay:        appContext.Config.GetHttpCalldelay(),
			Versuche:         appContext.Config.GetHttpVersuche(),
			WithProxy:        appContext.Config.GetHttpWithproxy(),
			WartezeitOnRetry: appContext.Config.GetHttpWartezeitonretry(),
		}
	}

	if appContext.publisher == nil {
		client, err := pubsub.NewClient(appContext.context, appContext.Config.GetProjectId())
		if err != nil {
			return errors.Wrap(err, "error creating publisher")
		}
		appContext.publisher = client
	}

	return nil
}
