CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS public.images (
	file_name varchar NOT NULL,
	created_date timestamptz NOT NULL DEFAULT now(),
	file_owner int4 NOT NULL,
	CONSTRAINT images_un UNIQUE (file_name)
);

CREATE TABLE IF NOT EXISTS  public.accounts (
	upload_token uuid NOT NULL DEFAULT uuid_generate_v4(),
	id serial4 NOT NULL,
	CONSTRAINT accounts_pk PRIMARY KEY (id),
	CONSTRAINT accounts_un UNIQUE (upload_token)
);