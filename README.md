<h1 align="center">Imagehost</h1>

Simple imagehost written in Golang

Main page             | Account page             | Admin page
:-------------------------:|:-------------------------:|:-------------------------:
![CleanShot 2025-06-22 at 17 58 36@2x](https://github.com/user-attachments/assets/521c2d2d-b062-4758-9f9a-c7a847be13e5)  |  ![CleanShot 2025-06-22 at 17 59 34@2x](https://github.com/user-attachments/assets/e40a8d60-4d43-4e63-8d56-700e2f963cbc) | ![CleanShot 2025-06-22 at 18 00 01@2x](https://github.com/user-attachments/assets/264aeac4-c926-45ae-9b59-8c49ce5467b1)

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