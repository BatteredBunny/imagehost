package main

import (
	"database/sql"
	"encoding/json"
)

const accountRolesEnumCreation = `
		DO $$
		BEGIN
    		IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'account_type') THEN
        		CREATE TYPE account_type AS ENUM ('USER', 'ADMIN');
    		END IF;
		END
		$$;
`

const imagesTableCreation = `
		CREATE TABLE IF NOT EXISTS public.images (
			file_name varchar NOT NULL, 
			created_date timestamptz NOT NULL DEFAULT now(), 
			file_uploader integer NOT NULL,
			CONSTRAINT images_un UNIQUE (file_name)
		);
`

const accountsTableCreation = `
		CREATE TABLE IF NOT EXISTS public.accounts (
			token uuid NOT NULL DEFAULT uuid_generate_v4(), 
			upload_token uuid NOT NULL DEFAULT uuid_generate_v4(), 
			id serial4 NOT NULL, 
			account_type account_type NOT NULL DEFAULT 'USER'::account_type, 
			CONSTRAINT accounts_pk PRIMARY KEY (id), 
			CONSTRAINT accounts_un UNIQUE (upload_token)
		);
`

const firstAccountCreationQuery = `
INSERT INTO public.accounts (id, account_type) values (1, 'ADMIN'::account_type) RETURNING *;
`

const postgresExtensionQuery = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
`

func (app *Application) prepareDb() {
	var err error

	app.db, err = sql.Open("postgres", app.config.PostgresConnectionString)
	if err != nil {
		app.logInfo.Fatal(err)
	}

	if _, err = app.db.Exec(accountRolesEnumCreation); err != nil {
		app.logInfo.Fatal(err)
	}

	if _, err = app.db.Exec(postgresExtensionQuery); err != nil {
		app.logInfo.Fatal(err)
	}

	if _, err = app.db.Exec(imagesTableCreation); err != nil {
		app.logInfo.Fatal(err)
	}

	if _, err = app.db.Exec(accountsTableCreation); err != nil {
		app.logInfo.Fatal(err)
	}

	var user User
	if app.db.QueryRow(firstAccountCreationQuery).Scan(&user.Token, &user.UploadToken, &user.ID, &user.AccountType) != nil {
		return
	}

	if data, err := json.MarshalIndent(user, "", "\t"); err != nil {
		app.logError.Fatal(err)
	} else {
		app.logInfo.Println("Created first account: ", string(data))
	}
}