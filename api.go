package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/h2non/filetype"
)

func is_valid_token(db *sql.DB, token string) (bool, int) {
	var id int
	if db.QueryRow("SELECT id FROM accounts WHERE token=$1", token).Scan(&id) != nil {
		return false, 0
	}

	return true, id
}

func is_valid_upload_token(db *sql.DB, token string) (bool, int) {
	var id int
	if db.QueryRow("SELECT id FROM accounts WHERE upload_token=$1", token).Scan(&id) != nil {
		return false, 0
	}

	return true, id
}

func file_exists(db *sql.DB, file_name string) bool {
	if db.QueryRow("SELECT file_name FROM public.images WHERE file_name=$1", file_name).Scan() != nil {
		return false
	}

	return true
}

// Api for deleting your own account
func account_delete_api(w http.ResponseWriter, r *http.Request, db *sql.DB, config Config) {
	if !r.Form.Has("token") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")

	result, user_id := is_valid_token(db, token)
	if !result {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	// Gets all images from account
	rows, err := db.Query("SELECT file_name FROM public.images WHERE file_owner=$1", user_id)
	if err != nil { // Im guessing this happens when it gets no results
		return
	}

	for rows.Next() {
		var file_name string
		rows.Scan(&file_name)

		delete_file(config, file_name)
	}

	db.Exec("DELETE FROM public.images WHERE file_owner=$1", user_id)
}

// Api for deleteing 1 image from your account
func delete_image_api(w http.ResponseWriter, r *http.Request, db *sql.DB, config Config) {
	if !r.Form.Has("upload_token") || !r.Form.Has("file_name") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	upload_token := r.FormValue("upload_token")
	file_name := r.FormValue("file_name")

	// Makes sure the upload token is valid
	result, user_id := is_valid_upload_token(db, upload_token)
	if !result {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	// Makes sure the image exists
	if !file_exists(db, file_name) {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	delete_file(config, file_name)
	db.Exec("DELETE FROM public.images WHERE file_name=$1 AND file_owner=$2", file_name, user_id)
	fmt.Fprintln(w, "Successfully deleted image")
}

// Api for uploading image
func upload_image_api(w http.ResponseWriter, r *http.Request, db *sql.DB, config Config, logger *log.Logger) {
	if !r.Form.Has("upload_token") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	upload_token := r.FormValue("upload_token")

	// Makes sure the token is valid
	result, user_id := is_valid_upload_token(db, upload_token)
	if !result {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	fileRaw, _, err := r.FormFile("file")
	if err != nil { // This error occurs when user doesn't send anything on file
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}

	file, err := io.ReadAll(fileRaw) // Reads the file into file variable
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if filetype.IsApplication(file) {
		http.Error(w, "Unsupported file type", http.StatusUnsupportedMediaType)
		return
	}

	extension, err := filetype.Get(file)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	full_file_name := generate_file_name(config.File_name_length) + "." + extension.Extension

	if config.s3client == nil { // Uploads to local storage
		if err := os.WriteFile(config.Data_folder+full_file_name, file, 0644); err != nil {
			logger.Fatal(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else { // Uploads to bucket
		if _, err := config.s3client.PutObject(&s3.PutObjectInput{
			Body:   bytes.NewReader(file),
			Bucket: aws.String(config.S3.Bucket),
			Key:    aws.String(full_file_name),
		}); err != nil {
			logger.Fatal(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	if _, err = db.Query(`INSERT INTO public.images (file_name, file_owner) VALUES ($1, $2)`, full_file_name, user_id); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	fileRaw.Close()

	http.Redirect(w, r, "https://"+r.Host+"/"+full_file_name, http.StatusFound)
	fmt.Fprintln(w, "https://"+r.Host+"/"+full_file_name)
}

// Api for changing your upload token
func new_upload_token_api(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if !r.Form.Has("token") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")

	if result, _ := is_valid_token(db, token); !result {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	var new_token string
	if db.QueryRow("UPDATE accounts SET upload_token=uuid_generate_v4() WHERE token=$1 RETURNING upload_token", token).Scan(&new_token) != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	fmt.Fprintln(w, new_token)
}
