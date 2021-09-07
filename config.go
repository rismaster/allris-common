package allris_common

import "time"

type Config interface {
	GetProxySecretHeaderKey() string
	GetProxyHostHeaderKey() string
	GetProxySecret() string
	GetProxyUrl() string
	GetProxyHost() string

	GetProjectId() string
	GetBucketFetched() string
	GetBucketBackup() string
	GetMinAgeBeforeDownload() time.Duration

	GetHttpTimeout() time.Duration
	GetHttpCalldelay() time.Duration
	GetHttpVersuche() int
	GetHttpWithproxy() bool
	GetHttpWartezeitonretry() time.Duration

	GetTimezone() string
	GetDateFormatWithTime() string

	GetPathToParse() string

	GetEntityTop() string
	GetEntityAnlage() string
	GetEntitySitzung() string
	GetAnlageType() string
	GetUrlAnlagedoc() string
	GetAnlageDocumentType() string

	GetTopFolder() string
	GetSitzungenFolder() string
	GetVorlagenFolder() string

	GetSitzungType() string
	GetVorlageType() string

	GetAlleSitzungenType() string

	GetDateFormatTech() string
	GetEntityTermin() string

	GetEntityVorlage() string
	GetDateFormat() string
}
