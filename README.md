<h1 align="center">Imagehost</h1>

Simple imagehost written in Golang

Main page             | Account page             | File modal              | Admin page
:-------------------------:|:-------------------------:|:-------------------------:|:-------------------------:
![screenshot](https://github.com/user-attachments/assets/c74b9b8d-8a0f-4322-b44a-745d229c4710)  |  ![screenshot](https://github.com/user-attachments/assets/1bb29030-b5e1-4c5e-a8d4-0ae94b252435)  |  ![screenshot](https://github.com/user-attachments/assets/32c43f99-4a21-4fee-ab3d-b675fc6d903e)  |  ![screenshot](https://github.com/user-attachments/assets/34cdfcde-e69d-4846-b2ac-68bfd576a1c1)

# Features
- Easy social login via github
- Account invite codes for enrolling new users
- Image automatic deletion
- Seperate upload tokens for automation setups (e.g scripts)
- Store data locally or on a S3/B2 bucket
- Sqlite and postgresql support
- View tracking

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
        openFirewall = false;
        settings.database_type = "postgresql";
    };

    postgresql.enable = true;
};
```

## Setup with docker

Have a look at docker-compose.yml

# Development

## Dev setup with nix

```
nix run .#test-service.driverInteractive
# Then visit http://localhost:8080
```