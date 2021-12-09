package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/dchest/uniuri"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"

	"mime/multipart"
)

const DATA_FOLDER = "/app/data/"
const FILE_NAME_LENGTH = 10
const MAX_UPLOAD_SIZE = 1024 * 1024 * 100

func main() {
	db, err := sql.Open("postgres", os.Getenv("POSTGRES_CONN"))
	if err != nil {
		log.Fatal(err)
		return
	}

	db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`)
	db.Exec("CREATE TABLE IF NOT EXISTS public.images (file_name varchar NOT NULL, created_date timestamptz NOT NULL DEFAULT now(), file_owner int4 NOT NULL, CONSTRAINT images_un UNIQUE (file_name));")
	db.Exec("CREATE TABLE IF NOT EXISTS  public.accounts (upload_token uuid NOT NULL DEFAULT uuid_generate_v4(), id serial4 NOT NULL, CONSTRAINT accounts_pk PRIMARY KEY (id), CONSTRAINT accounts_un UNIQUE (upload_token));")

	go auto_deletion()

	r := mux.NewRouter()
	r.PathPrefix("/favicon.ico").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "/app/public/favicon.ico")
	})

	r.PathPrefix("/").Handler(http.StripPrefix("/", middleware(http.FileServer(http.Dir(DATA_FOLDER)), db)))

	fmt.Println("Starting server")
	log.Fatal(http.ListenAndServe(":80", r))
}

func middleware(h http.Handler, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			api(w, r, db)
			return
		} else if r.URL.Path == "" {
			tmpl, err := template.New("index.html").ParseFiles("/app/template/index.html")

			if err != nil {
				log.Fatal(err)
				return
			}

			tmpl.Execute(w, r.Host)
			return
		}

		_, err := os.Open(DATA_FOLDER + path.Clean(r.URL.Path))
		if os.IsNotExist(err) {
			http.Redirect(w, r, "https://www.youtube.com/watch?v=dQw4w9WgXcQ", http.StatusFound)
			return
		}

		h.ServeHTTP(w, r)
	})
}

func get_extension(fileHeader *multipart.FileHeader) (string, error) {
	headerRaw, err := fileHeader.Open()
	if err != nil {
		log.Fatal(err)
	}

	header, err := io.ReadAll(headerRaw)
	if err != nil {
		log.Fatal(err)
	}

	mimetype := http.DetectContentType(header)

	headerRaw.Close()

	switch mimetype {
	case "image/jpeg":
		return ".jpg", nil
	case "image/png":
		return ".png", nil
	case "image/gif":
		return ".gif", nil
	case "image/webp":
		return ".webp", nil
	case "video/mp4":
		return ".mp4", nil
	case "video/webm":
		return ".webm", nil
	case "application/ogg":
		return ".ogg", nil
	default:
		return "", fmt.Errorf("Unsupported file type: %s", mimetype)
	}
}

func generate_file_name() string {
	return uniuri.NewLenChars(FILE_NAME_LENGTH, []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"))
}
