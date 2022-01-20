package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/dchest/uniuri"
	_ "github.com/lib/pq"
)

type User struct {
	Upload_token string
	Token        string
	Id           int
	Account_type string
}

type s3_config struct {
	Access_key_id     string `json:"access_key_id"`
	Secret_access_key string `json:"secret_access_key"`
	Bucket            string `json:"bucket"`
	Region            string `json:"region"`
	Endpoint          string `json:"endpoint"`
	CDN_Domain        string `json:"cdn_domain"`
}
type Config struct {
	Template_folder            string `json:"template_folder"`
	Static_folder              string `json:"static_folder"`
	Data_folder                string `json:"data_folder"`
	File_name_length           int    `json:"file_name_length"`
	Max_upload_size            int    `json:"max_upload_size"`
	Postgres_connection_string string `json:"postgres_connection_string"`
	Port                       string `json:"port"`

	S3       s3_config `json:"s3"`
	s3client *s3.S3
}

func main() {
	logger := log.Default()

	var config_location string
	flag.StringVar(&config_location, "c", "config.json", "Location of config file")
	flag.Parse()

	raw, err := os.ReadFile(config_location)
	if err != nil {
		logger.Fatal(err)
	}

	var config Config
	if json.Unmarshal(raw, &config) != nil {
		logger.Fatal(err)
	}

	if config.S3 == (s3_config{}) {
		logger.Println("Storing files in", config.Data_folder)

		if file, _ := os.Stat(config.Data_folder); file == nil {
			logger.Println("Creating data folder")

			if err := os.Mkdir(config.Data_folder, 0777); err != nil {
				logger.Fatal(err)
			}
		}
	} else {
		logger.Println("Storing files in s3 bucket")
		config.s3client = prepeare_s3(config)
	}

	var db *sql.DB = prepeare_db(config, logger)

	go auto_deletion(db, config, logger)

	apiListTemplate, err := template.New("api_list.html").ParseFiles(config.Template_folder + "api_list.html")
	if err != nil {
		logger.Fatal(err)
	}

	http.HandleFunc("/api_list", func(w http.ResponseWriter, r *http.Request) {
		if apiListTemplate.Execute(w, r.Host) != nil {
			fmt.Fprintf(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			r.Body = http.MaxBytesReader(w, r.Body, int64(config.Max_upload_size))

			if r.ParseMultipartForm(int64(config.Max_upload_size)) != nil {
				fmt.Fprintf(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
				return
			}

			switch r.URL.Path {
			case "/api/upload":
				upload_image_api(w, r, db, config, logger)
			case "/api/delete":
				delete_image_api(w, r, db, config)
			case "/api/account/new_upload_token":
				new_upload_token_api(w, r, db)
			case "/api/account/delete":
				account_delete_api(w, r, db, config)
			case "/api/admin/create_user":
				admin_create_user(w, r, db)
			case "/api/admin/delete_user":
				admin_delete_user(w, r, db, config)
			default:
				http.NotFound(w, r)
			}
		} else {
			http.NotFound(w, r)
		}
	})

	http.Handle("/public/",
		http.StripPrefix("/public/", http.FileServer(http.Dir(config.Static_folder))),
	)

	indexTemplate, err := template.New("index.html").ParseFiles(config.Template_folder + "index.html")
	if err != nil {
		logger.Fatal(err)
	}

	http.Handle("/", middleware(indexTemplate, db, config, logger))

	logger.Printf("Starting server at http://localhost:%s\n", config.Port)
	logger.Fatal(http.ListenAndServe(":"+config.Port, nil))
}

func middleware(indexTemplate *template.Template, db *sql.DB, config Config, logger *log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Println(r.URL.Path)

		if r.URL.Path == "/" {
			if indexTemplate.Execute(w, r.Host) != nil {
				fmt.Fprintf(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}

			return
		}

		// Looks in database for uploaded file
		if db.QueryRow("SELECT file_name FROM public.images WHERE file_name=$1;", path.Base(r.URL.Path)).Scan() != nil {
			http.Redirect(w, r, "https://www.youtube.com/watch?v=dQw4w9WgXcQ", http.StatusFound)
			return
		}

		if config.s3client == nil {
			http.ServeFile(w, r, config.Data_folder+path.Clean(r.URL.Path))
		} else {
			http.Redirect(w, r, "https://"+config.S3.CDN_Domain+"/file/"+config.S3.Bucket+path.Clean(r.URL.Path), http.StatusFound)
		}
	})
}

// Deletes a file
func delete_file(config Config, file_name string) {
	if config.s3client == nil { // Deletes from local storage
		os.Remove(config.Data_folder + file_name)
	} else { // Delete from s3
		config.s3client.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(config.S3.Bucket),
			Key:    aws.String(file_name),
		})
	}
}

// Gets s3 session from config
func prepeare_s3(config Config) *s3.S3 {
	return s3.New(session.New(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(config.S3.Access_key_id, config.S3.Secret_access_key, ""),
		Endpoint:         aws.String(config.S3.Endpoint),
		Region:           aws.String(config.S3.Region),
		S3ForcePathStyle: aws.Bool(true),
	}))
}

// Makes sure db is correctly setup and connects to it
func prepeare_db(config Config, logger *log.Logger) *sql.DB {
	db, err := sql.Open("postgres", config.Postgres_connection_string)
	if err != nil {
		logger.Fatal(err)
	}

	if _, err := db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`); err != nil {
		logger.Fatal(err)
	}

	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS public.images (file_name varchar NOT NULL, created_date timestamptz NOT NULL DEFAULT now(), file_owner int4 NOT NULL, CONSTRAINT images_un UNIQUE (file_name));"); err != nil {
		logger.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS public.accounts (token uuid NOT NULL DEFAULT uuid_generate_v4(), upload_token uuid NOT NULL DEFAULT uuid_generate_v4(), id serial4 NOT NULL, account_type text NOT NULL DEFAULT 'USER', CONSTRAINT accounts_pk PRIMARY KEY (id), CONSTRAINT accounts_un UNIQUE (upload_token));"); err != nil {
		logger.Fatal(err)
	}

	return db
}

func generate_file_name(file_name_length int) string {
	return uniuri.NewLenChars(file_name_length, []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"))
}
