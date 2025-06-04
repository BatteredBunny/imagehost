package cmd

import (
	"net/http"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/didip/tollbooth/v8/limiter"
	"github.com/gin-gonic/gin"
	"github.com/go-co-op/gocron/v2"
	"github.com/rs/zerolog/log"
)

type Application struct {
	config      Config
	db          Database
	s3client    *s3.S3
	RateLimiter *limiter.Limiter
	cron        gocron.Scheduler

	Router *gin.Engine
}

type fileStorageMethod string

const (
	fileStorageLocal fileStorageMethod = "LOCAL"
	fileStorageS3    fileStorageMethod = "S3"
)

type Config struct {
	DataFolder            string `toml:"data_folder"`
	MaxUploadSize         int64  `toml:"max_upload_size"`
	DatabaseType          string `toml:"database_type"`
	DatabaseConnectionUrl string `toml:"database_connection_url"`
	Port                  string `toml:"port"`

	behindReverseProxy bool   `toml:"behind_reverse_proxy"`
	trustedProxy       string `toml:"trusted_proxy"`
	publicUrl          string `toml:"public_url"` // URL to use for github callback

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
	log.Info().Msgf("Starting server at http://localhost:%s", app.config.Port)
	log.Fatal().Err(http.ListenAndServe(":"+app.config.Port, app.Router)).Msg("HTTP server failed")
}
