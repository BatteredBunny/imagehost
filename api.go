package main

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
)

func api(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	r.ParseMultipartForm(MAX_UPLOAD_SIZE)

	if r.ContentLength > MAX_UPLOAD_SIZE {
		http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
		return
	}

	if !r.Form.Has("token") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	switch r.FormValue("type") {
	case "upload":
		upload_api(w, r, db)
	case "delete":
		delete_image_api(w, r, db)
	}

}

func delete_image_api(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	upload_token := r.FormValue("token")
	file_name := r.FormValue("file_name")

	var token_result sql.NullString
	err := db.QueryRow(`SELECT id FROM public.accounts WHERE upload_token = $1`, upload_token).Scan(&token_result)
	if err != nil { // This error occurs when the token is incorrect
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	if !token_result.Valid {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	rows, err := db.Query("SELECT file_name FROM public.images WHERE file_name=$1 and file_owner=$2;", file_name, token_result.String)
	if err != nil { // Im guessing this happens when it gets no results
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	for rows.Next() {
		var file_name string
		rows.Scan(&file_name)

		os.Remove("/app/data/" + file_name)
	}

	fmt.Fprintf(w, "Successfully deleted image")
}

func upload_api(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	upload_token := r.FormValue("token")

	var result sql.NullString
	err := db.QueryRow(`SELECT id FROM public.accounts WHERE upload_token = $1`, upload_token).Scan(&result)
	if err != nil { // This error occurs when the token is incorrect
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	if !result.Valid {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	fileRaw, fileHeader, err := r.FormFile("file")
	if err != nil { // This error occurs when user doesn't send anything on file
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		fmt.Println("no file")
		return
	}

	file, err := io.ReadAll(fileRaw) // Reads the file into file variable
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	extension, err := get_extension(fileHeader)
	if err != nil { // Wrong file type
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
		return
	}

	full_file_name := generate_file_name() + extension

	err = os.WriteFile(DATA_FOLDER+full_file_name, file, 0644)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	_, err = db.Query(`INSERT INTO public.images (file_name, file_owner) VALUES ($1, $2)`, full_file_name, result.String)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	fileRaw.Close()

	http.Redirect(w, r, "https://"+r.Host+"/"+full_file_name, http.StatusFound)
	fmt.Fprintln(w, "https://"+r.Host+"/"+full_file_name)
}
