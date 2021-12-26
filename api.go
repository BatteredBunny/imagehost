package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/h2non/filetype"
)

// Deletes your account with images
func account_delete_api(w http.ResponseWriter, r *http.Request, db *sql.DB, config Config, s3client *s3.S3) {
	if !r.Form.Has("token") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")

	var id int
	row := db.QueryRow("DELETE FROM accounts WHERE token=$1 RETURNING id", token)
	if row.Scan(&id) == sql.ErrNoRows {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	rows, err := db.Query("SELECT file_name FROM public.images WHERE file_owner=$1", id)
	if err != nil { // Im guessing this happens when it gets no results
		return
	}

	for rows.Next() {
		var file_name string
		rows.Scan(&file_name)

		s3client.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(config.S3.Bucket),
			Key:    aws.String(file_name),
		})
	}

	db.Exec("DELETE FROM public.images WHERE file_owner=$1", id)
}

func delete_image_api(w http.ResponseWriter, r *http.Request, db *sql.DB, config Config, s3client *s3.S3) {
	if !r.Form.Has("upload_token") || !r.Form.Has("file_name") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	upload_token := r.FormValue("upload_token")
	file_name := r.FormValue("file_name")

	var token_result sql.NullString
	if db.QueryRow(`SELECT id FROM public.accounts WHERE upload_token = $1`, upload_token).Scan(&token_result) != nil { // This error occurs when the token is incorrect
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	} else if !token_result.Valid {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	if db.QueryRow("SELECT file_name FROM public.images WHERE file_name=$1 AND file_owner=$2;", file_name, token_result.String).Scan() == sql.ErrNoRows {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	s3client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(config.S3.Bucket),
		Key:    aws.String(file_name),
	})

	db.Exec("DELETE FROM public.images WHERE file_name=$1 AND file_owner=$2", file_name, token_result.String)
	fmt.Fprintln(w, "Successfully deleted image")
}

func upload_image_api(w http.ResponseWriter, r *http.Request, db *sql.DB, config Config, s3client *s3.S3) {
	if !r.Form.Has("upload_token") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	upload_token := r.FormValue("upload_token")

	var result sql.NullString
	if db.QueryRow("SELECT id FROM public.accounts WHERE upload_token=$1", upload_token).Scan(&result) != nil { // This error occurs when the token is incorrect
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	} else if !result.Valid {
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

	if _, err := s3client.PutObject(&s3.PutObjectInput{
		Body:   bytes.NewReader(file),
		Bucket: aws.String(config.S3.Bucket),
		Key:    aws.String(full_file_name),
	}); err != nil {
		log.Fatal(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if _, err = db.Query(`INSERT INTO public.images (file_name, file_owner) VALUES ($1, $2)`, full_file_name, result.String); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	fileRaw.Close()

	http.Redirect(w, r, "https://"+r.Host+"/"+full_file_name, http.StatusFound)
	fmt.Fprintln(w, "https://"+r.Host+"/"+full_file_name)
}

func new_upload_token_api(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if !r.Form.Has("token") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")

	var new_token string
	if db.QueryRow("UPDATE accounts SET upload_token=uuid_generate_v4() WHERE token=$1 RETURNING upload_token", token).Scan(&new_token) == sql.ErrNoRows {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	fmt.Fprintln(w, new_token)
}
