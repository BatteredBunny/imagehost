# Imagehost

The program has only one flag, -c, which specifies config file used. All other settings are in the config file.

## Docker

Use example_docker.toml

## Without docker

Use example_local.toml

## S3/B2 bucket

Look in example_s3.toml for what settings to add to your config.

# Dev setup with docker

Uncomment ``./example_docker.toml:/app/config.toml`` in docker-compose.yml

Run ``docker compose up --build`` and then visit http://localhost:8080/

Thats it!

# Basic nixos setup

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