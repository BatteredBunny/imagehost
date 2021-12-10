package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/dchest/uniuri"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

const DATA_FOLDER = "/app/data/"
const FILE_NAME_LENGTH = 10
const MAX_UPLOAD_SIZE = 1024 * 1024 * 100

type User struct {
	Upload_token string
	Token        string
	Id           int
	Account_type string
}

func main() {
	db, err := sql.Open("postgres", os.Getenv("POSTGRES_CONN"))
	if err != nil {
		log.Fatal(err)
		return
	}

	db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`)
	db.Exec("CREATE TABLE IF NOT EXISTS public.images (file_name varchar NOT NULL, created_date timestamptz NOT NULL DEFAULT now(), file_owner int4 NOT NULL, CONSTRAINT images_un UNIQUE (file_name));")
	db.Exec("CREATE TABLE IF NOT EXISTS public.accounts (token uuid NOT NULL DEFAULT uuid_generate_v4(), upload_token uuid NOT NULL DEFAULT uuid_generate_v4(), id serial4 NOT NULL, account_type text NOT NULL DEFAULT 'USER', CONSTRAINT accounts_pk PRIMARY KEY (id), CONSTRAINT accounts_un UNIQUE (upload_token));")

	go auto_deletion(db)

	r := mux.NewRouter()
	r.PathPrefix("/public").Handler(http.StripPrefix("/public", http.FileServer(http.Dir("/app/public"))))
	r.PathPrefix("/").Handler(http.StripPrefix("/", middleware(http.FileServer(http.Dir(DATA_FOLDER)), db)))

	fmt.Println("Starting server")
	log.Fatal(http.ListenAndServe(":80", r))
}

func middleware(h http.Handler, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "api/") {
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
		} else if r.URL.Path == "api_list" {
			tmpl, err := template.New("api_list.html").ParseFiles("/app/template/api_list.html")

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

func generate_file_name() string {
	return uniuri.NewLenChars(FILE_NAME_LENGTH, []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"))
}
