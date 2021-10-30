CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE public.images (
	file_name varchar NOT NULL,
	created_date timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT images_un UNIQUE (file_name)
);

CREATE TABLE public.accounts (
	upload_token uuid NOT NULL DEFAULT uuid_generate_v4(),
	id serial4 NOT NULL,
	CONSTRAINT accounts_pk PRIMARY KEY (id),
	CONSTRAINT accounts_un UNIQUE (upload_token)
);