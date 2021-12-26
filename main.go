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
	"time"

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

type Config struct {
	Template_folder            string `json:"template_folder"`
	Static_folder              string `json:"static_folder"`
	File_name_length           int    `json:"file_name_length"`
	Max_upload_size            int    `json:"max_upload_size"`
	Postgres_connection_string string `json:"postgres_connection_string"`
	Port                       string `json:"port"`

	S3 struct {
		Access_key_id     string `json:"access_key_id"`
		Secret_access_key string `json:"secret_access_key"`
		Bucket            string `json:"bucket"`
		Region            string `json:"region"`
		Endpoint          string `json:"endpoint"`
		CDN_Domain        string `json:"cdn_domain"`
	}
}

func main() {
	var config_location string
	flag.StringVar(&config_location, "c", "config.json", "Location of config file")
	flag.Parse()

	raw, err := os.ReadFile(config_location)
	if err != nil {
		log.Fatal(err)
	}

	var config Config
	if json.Unmarshal(raw, &config) != nil {
		log.Fatal(err)
	}

	var s3client *s3.S3 = prepeare_s3(config)
	var db *sql.DB = prepeare_db(config)

	go auto_deletion(db, config, s3client)

	apiListTemplate, err := template.New("api_list.html").ParseFiles(config.Template_folder + "api_list.html")
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/api_list", func(w http.ResponseWriter, r *http.Request) {
		if apiListTemplate.Execute(w, r.Host) != nil {
			fmt.Fprintf(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			r.ParseMultipartForm(int64(config.Max_upload_size))

			if r.ContentLength > int64(config.Max_upload_size) {
				http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
				return
			}

			switch r.URL.Path {
			case "/api/upload":
				upload_image_api(w, r, db, config, s3client)
			case "/api/delete":
				delete_image_api(w, r, db, config, s3client)
			case "/api/account/new_upload_token":
				new_upload_token_api(w, r, db)
			case "/api/account/delete":
				account_delete_api(w, r, db, config, s3client)
			case "/api/admin/create_user":
				admin_create_user(w, r, db)
			case "/api/admin/delete_user":
				admin_delete_user(w, r, db, config, s3client)
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
		log.Fatal(err)
	}

	http.Handle("/", middleware(indexTemplate, db, config))

	fmt.Printf("%s | Starting server at http://localhost:%s\n", time.Now().Format(time.RFC3339), config.Port)
	log.Fatal(http.ListenAndServe(":"+config.Port, nil))
}

func middleware(indexTemplate *template.Template, db *sql.DB, config Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s | %s\n", time.Now().Format(time.RFC3339), r.URL.Path)

		if r.URL.Path == "/" {
			if indexTemplate.Execute(w, r.Host) != nil {
				fmt.Fprintf(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}

			return
		}

		if db.QueryRow("SELECT file_name FROM public.images WHERE file_name=$1;", path.Base(r.URL.Path)).Scan() == sql.ErrNoRows {
			http.Redirect(w, r, "https://www.youtube.com/watch?v=dQw4w9WgXcQ", http.StatusFound)
			return
		}

		http.Redirect(w, r, "https://"+config.S3.CDN_Domain+"/file/"+config.S3.Bucket+path.Clean(r.URL.Path), http.StatusFound)
	})
}

func prepeare_s3(config Config) *s3.S3 {
	return s3.New(session.New(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(config.S3.Access_key_id, config.S3.Secret_access_key, ""),
		Endpoint:         aws.String(config.S3.Endpoint),
		Region:           aws.String(config.S3.Region),
		S3ForcePathStyle: aws.Bool(true),
	}))
}

func prepeare_db(config Config) *sql.DB {
	db, err := sql.Open("postgres", config.Postgres_connection_string)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`); err != nil {
		log.Fatal(err)
	}

	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS public.images (file_name varchar NOT NULL, created_date timestamptz NOT NULL DEFAULT now(), file_owner int4 NOT NULL, CONSTRAINT images_un UNIQUE (file_name));"); err != nil {
		log.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS public.accounts (token uuid NOT NULL DEFAULT uuid_generate_v4(), upload_token uuid NOT NULL DEFAULT uuid_generate_v4(), id serial4 NOT NULL, account_type text NOT NULL DEFAULT 'USER', CONSTRAINT accounts_pk PRIMARY KEY (id), CONSTRAINT accounts_un UNIQUE (upload_token));"); err != nil {
		log.Fatal(err)
	}

	return db
}

func generate_file_name(file_name_length int) string {
	return uniuri.NewLenChars(file_name_length, []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"))
}
