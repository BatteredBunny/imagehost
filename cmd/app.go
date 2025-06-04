package cmd

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/rs/zerolog/log"
)

var ErrUnknownStorageMethod = errors.New("unknown file storage method")

//go:embed public/*
var publicFiles embed.FS

func PublicFiles() http.FileSystem {
	sub, err := fs.Sub(publicFiles, "public")
	if err != nil {
		panic(err)
	}

	return http.FS(sub)
}

func prepareStorage(c Config) (s3client *s3.S3) {
	switch c.fileStorageMethod {
	case fileStorageS3:
		log.Info().Msg("Storing files in s3 bucket")

		if s3session, err := session.NewSession(&aws.Config{
			Credentials:      credentials.NewStaticCredentials(c.S3.AccessKeyID, c.S3.SecretAccessKey, ""),
			Endpoint:         aws.String(c.S3.Endpoint),
			Region:           aws.String(c.S3.Region),
			S3ForcePathStyle: aws.Bool(true),
		}); err != nil {
			log.Fatal().Err(err).Msg("Failed to create s3 session")
		} else {
			s3client = s3.New(s3session)
		}
	case fileStorageLocal:
		log.Info().Msgf("Storing files in %s", c.DataFolder)

		if file, _ := os.Stat(c.DataFolder); file == nil {
			log.Info().Msg("Creating data folder")

			if err := os.Mkdir(c.DataFolder, 0770); err != nil {
				log.Fatal().Err(err).Msg("Failed to create data folder")
			}
		}
	default:
		log.Fatal().Err(ErrUnknownStorageMethod).Msg("Can't setup storage, none selected")
	}

	return
}

func initializeConfig() (c Config) {
	var configLocation string
	flag.StringVar(&configLocation, "c", "config.toml", "Location of config file")
	flag.Parse()

	rawConfig, err := os.ReadFile(configLocation)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open config file")
	}

	if _, err = toml.Decode(string(rawConfig), &c); err != nil {
		log.Fatal().Err(err).Msg("Can't parse config file")
	}

	if c.S3 != (s3Config{}) {
		c.fileStorageMethod = fileStorageS3
	} else {
		c.fileStorageMethod = fileStorageLocal
	}

	if c.publicUrl == "" {
		log.Warn().Msg("Warning no public_url option set in toml, github login might not work")
		c.publicUrl = fmt.Sprintf("http://localhost:%s", c.Port)
	}

	return
}
