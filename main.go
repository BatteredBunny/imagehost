package main

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"

	"github.com/dchest/uniuri"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"

	"mime/multipart"
)

const DATA_FOLDER = "/app/data/"
const MAX_UPLOAD_SIZE = 1024 * 1024 * 100 // 100MB
const CONNECTION_STRING = "host=db port=5432 user=postgres password=123 dbname=imagehost sslmode=disable"
const FILE_NAME_LENGTH = 10

func main() {
	go auto_deletion()

	r := mux.NewRouter()
	r.PathPrefix("/").Handler(http.StripPrefix("/", middleware(http.FileServer(http.Dir(DATA_FOLDER)))))

	fmt.Println("Starting server")
	log.Fatal(http.ListenAndServe(":80", r))
}

func middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			r.ParseMultipartForm(MAX_UPLOAD_SIZE)

			if r.ContentLength > MAX_UPLOAD_SIZE {
				http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
				return
			}

			if !r.Form.Has("token") {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}

			upload_token := r.FormValue("token")

			db, err := sql.Open("postgres", CONNECTION_STRING)
			if err != nil { // This error occurs when it can't connect to database
				http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
				return
			}

			var result sql.NullString
			err = db.QueryRow(`SELECT id FROM public.accounts WHERE upload_token = $1`, upload_token).Scan(&result)
			if err != nil { // This error occurs when the token is incorrect
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			if result.Valid {
				fileRaw, fileHeader, err := r.FormFile("file")
				if err != nil { // This error occurs when user doesn't send anything on file
					http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
					return
				}

				data, err := io.ReadAll(fileRaw)
				if err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}

				extension, err := get_extension(fileHeader)
				if err != nil { // Wrong file type
					http.Error(w, http.StatusText(http.StatusUnsupportedMediaType), http.StatusUnsupportedMediaType)
					return
				}

				full_file_name := generate_file_name() + extension

				err = os.WriteFile(DATA_FOLDER+full_file_name, data, 0644)
				if err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}

				_, err = db.Query(`INSERT INTO public.images VALUES ($1)`, full_file_name)
				if err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}

				fileRaw.Close()

				fmt.Fprintln(w, "https://"+r.Host+"/"+full_file_name)
				return
			} else {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
		} else if r.URL.Path == "" {
			http.ServeFile(w, r, "/app/public/index.html")
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
	if mimetype != "image/png" && mimetype != "image/jpeg" && mimetype != "image/webp" {
		return "", errors.New("wrong mimetype")
	}

	headerRaw.Close()

	extensions, err := mime.ExtensionsByType(mimetype)
	if err != nil || len(extensions) == 0 {
		return "", errors.New("no extension found")
	}

	return extensions[len(extensions)-1], nil
}

func generate_file_name() string {
	return uniuri.NewLenChars(FILE_NAME_LENGTH, []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"))
}
