package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

func is_admin(db *sql.DB, token string) bool {
	var account_type string
	row := db.QueryRow("SELECT account_type FROM accounts WHERE token=$1 AND account_type='ADMIN'", token)
	if row.Scan(&account_type) == sql.ErrNoRows {
		return false
	}

	return true
}

func admin_create_user(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if !r.Form.Has("token") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")

	if !is_admin(db, token) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	var new_user User
	row := db.QueryRow("INSERT INTO public.accounts DEFAULT values RETURNING token, upload_token, id, account_type")
	if row.Scan(&new_user.Token, &new_user.Upload_token, &new_user.Id, &new_user.Account_type) == sql.ErrNoRows {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	json, err := json.Marshal(new_user)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, string(json))
}

func admin_delete_user(w http.ResponseWriter, r *http.Request, db *sql.DB, config Config) {
	if !r.Form.Has("token") || !r.Form.Has("id") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	id := r.FormValue("id")

	if !is_admin(db, token) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	db.Exec("DELETE FROM public.accounts WHERE id=$1", id)

	rows, err := db.Query("SELECT file_name FROM public.images WHERE file_owner=$1", id)
	if err != nil { // Im guessing this happens when it gets no results
		return
	}

	for rows.Next() {
		var file_name string
		rows.Scan(&file_name)

		os.Remove(config.Data_folder + file_name)
	}

	db.Exec("DELETE FROM public.images WHERE file_owner=$1", id)

	fmt.Fprintf(w, "User %s deleted\n", id)
}
