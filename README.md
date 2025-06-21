<h1 align="center">Imagehost</h1>

Simple imagehost written in Golang

Main page             |  Account page
:-------------------------:|:-------------------------:
![CleanShot 2025-06-21 at 20 49 35@2x](https://github.com/user-attachments/assets/990a1be7-84a7-4df1-9fe8-067807580b28)  |  ![CleanShot 2025-06-21 at 20 49 07@2x](https://github.com/user-attachments/assets/a571c9f0-1aa2-477f-962c-627b6e900a94)

# Features
- Easy social login via github
- Account invite codes for enrolling new users
- Image automatic deletion
- Seperate upload codes for automation setups (e.g scripts)
- Store data locally or on a S3/B2 bucket
- Sqlite and postgresql support

# Usage

Deploy the service with either the nixos module or docker-compose then configure the service.

Have a look at the example configs in ``examples/``

# Config reference

TODO

# Setup
## Setup with nixos module

```nix
inputs = {
    imagehost.url = "github:BatteredBunny/imagehost";
};
```

```nix
imports = [ inputs.imagehost.nixosModules.default ];

services = {
    imagehost = {
        enable = true;
        createDbLocally = true;
        settings.database_type = "postgresql";
    };

    postgresql.enable = true;
};
```

## Setup with docker

Have a look at docker-compose.yml

# Development

## Dev setup with docker

Theres a docker-compose.yml config for setting up the service with postgresql

```
docker compose up --build
# Then visit http://localhost:8080
```