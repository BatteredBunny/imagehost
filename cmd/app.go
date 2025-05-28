package cmd

import (
	"embed"
	"errors"
	"flag"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

var ErrUnknownStorageMethod = errors.New("unknown file storage method")

//go:embed templates/*
var templateFiles embed.FS

//go:embed public/*
var publicFiles embed.FS

func PublicFiles() http.FileSystem {
	sub, err := fs.Sub(publicFiles, "public")
	if err != nil {
		panic(err)
	}

	return http.FS(sub)
}

func setupLogging() *Logger {
	flags := log.Ldate | log.Ltime | log.Lshortfile | log.Lmsgprefix

	return &Logger{
		logInfo:    log.New(os.Stdout, "INFO: ", flags),
		logError:   log.New(os.Stdout, "ERROR: ", flags),
		logWarning: log.New(os.Stdout, "WARN: ", flags),
	}
}

func prepareStorage(l *Logger, c Config) (s3client *s3.S3) {
	switch c.fileStorageMethod {
	case fileStorageS3:
		l.logInfo.Println("Storing files in s3 bucket")

		if s3session, err := session.NewSession(&aws.Config{
			Credentials:      credentials.NewStaticCredentials(c.S3.AccessKeyID, c.S3.SecretAccessKey, ""),
			Endpoint:         aws.String(c.S3.Endpoint),
			Region:           aws.String(c.S3.Region),
			S3ForcePathStyle: aws.Bool(true),
		}); err != nil {
			l.logInfo.Fatal(err)
		} else {
			s3client = s3.New(s3session)
		}
	case fileStorageLocal:
		l.logInfo.Println("Storing files in", c.DataFolder)

		if file, _ := os.Stat(c.DataFolder); file == nil {
			l.logInfo.Println("Creating data folder")

			if err := os.Mkdir(c.DataFolder, 0777); err != nil {
				l.logError.Fatal(err)
			}
		}
	default:
		l.logError.Fatal(ErrUnknownStorageMethod)
	}

	return
}

func initializeConfig(l *Logger) (c Config) {
	var configLocation string
	flag.StringVar(&configLocation, "c", "config.toml", "Location of config file")
	flag.Parse()

	rawConfig, err := os.ReadFile(configLocation)
	if err != nil {
		l.logError.Fatal(err)
	}

	if _, err = toml.Decode(string(rawConfig), &c); err != nil {
		l.logError.Fatal(err)
	}

	if c.S3 != (s3Config{}) {
		c.fileStorageMethod = fileStorageS3
	} else {
		c.fileStorageMethod = fileStorageLocal
	}

	return
}

func setupTemplates() *Templates {
	return &Templates{
		apiListTemplate: template.Must(template.New("api_list.gohtml").ParseFS(
			templateFiles,
			"templates/api_list.gohtml",
		)),
		indexTemplate: template.Must(template.New("index.gohtml").ParseFS(
			templateFiles,
			"templates/index.gohtml",
		)),
	}
}
