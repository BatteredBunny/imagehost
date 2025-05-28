package cmd

import (
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/didip/tollbooth/v8/limiter"
	"github.com/gin-gonic/gin"
)

type Application struct {
	*Logger
	config      Config
	db          Database
	s3client    *s3.S3
	RateLimiter *limiter.Limiter

	Router *gin.Engine
}

type fileStorageMethod string

const (
	fileStorageLocal fileStorageMethod = "LOCAL"
	fileStorageS3    fileStorageMethod = "S3"
)

type Logger struct {
	logError   *log.Logger
	logWarning *log.Logger
	logInfo    *log.Logger
}

type Config struct {
	DataFolder            string `toml:"data_folder"`
	MaxUploadSize         int64  `toml:"max_upload_size"`
	DatabaseType          string `toml:"database_type"`
	DatabaseConnectionUrl string `toml:"database_connection_url"`
	Port                  string `toml:"port"`

	behindReverseProxy bool   `toml:"behind_reverse_proxy"`
	trustedProxy       string `toml:"trusted_proxy"`

	fileStorageMethod fileStorageMethod
	S3                s3Config `toml:"s3"`
}

type s3Config struct {
	AccessKeyID     string `toml:"access_key_id"`
	SecretAccessKey string `toml:"secret_access_key"`
	Bucket          string `toml:"bucket"`
	Region          string `toml:"region"`
	Endpoint        string `toml:"endpoint"`
	CdnDomain       string `toml:"cdn_domain"`
}

func (app *Application) Run() {
	app.logInfo.Printf("Starting server at http://localhost:%s\n", app.config.Port)
	app.logError.Fatal(http.ListenAndServe(":"+app.config.Port, app.Router))
}
