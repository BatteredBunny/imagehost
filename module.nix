{ pkgs
, config ? pkgs.config
, lib ? pkgs.lib
, ...
}:
let
  cfg = config.services.imagehost;
  toml = pkgs.formats.toml { };
  tomlSetting = toml.generate "config.toml" cfg.settings;
in
{
  options.services.imagehost = {
    enable = lib.mkEnableOption "imagehost";

    package = lib.mkOption {
      description = "package to use";
      default = pkgs.callPackage ./build.nix { };
    };

    createDbLocally = lib.mkEnableOption "creation of database on the instance";

    settings = {
      port = lib.mkOption {
        type = lib.types.int;
        apply = toString;
        description = "port to run service on";
        default = 8872;
      };

      database_type = lib.mkOption {
        type = lib.types.str;
        default = "postgresql";
        description = "Database type";
      };

      database_connection_url = lib.mkOption {
        type = lib.types.str;
        default = "";
        description = "Database connection string";
      };

      max_upload_size = lib.mkOption {
        type = lib.types.int;
        default = 104857600;
        description = "Max upload size in bytes";
      };

      data_folder = lib.mkOption {
        type = lib.types.path;
        default = "/var/lib/imagehost/data";
        description = "Folder to store local image data in";
      };

      behind_reverse_proxy = lib.mkOption {
        type = lib.types.bool;
        default = false;
        description = "Allows using trusted proxy settings";
      };

      trusted_proxy = lib.mkOption {
        type = lib.types.str;
        default = "";
        description = "Which proxy to trust for IP information";
      };

      # s3 = {
      #   access_key_id
      #   secret_access_key
      #   bucket
      #   region
      #   endpoint
      #   cdn_domain
      # };
    };
  };

  config = lib.mkIf cfg.enable {
    systemd.services.imagehost = {
      enable = true;
      serviceConfig = {
        User = "imagehost";
        Group = "imagehost";
        ProtectSystem = "full";
        ProtectHome = "yes";
        DeviceAllow = [ "" ];
        LockPersonality = true;
        MemoryDenyWriteExecute = true;
        PrivateDevices = true;
        ProtectClock = true;
        ProtectControlGroups = true;
        ProtectHostname = true;
        ProtectKernelLogs = true;
        ProtectKernelModules = true;
        ProtectKernelTunables = true;
        ProtectProc = "invisible";
        RestrictNamespaces = true;
        RestrictRealtime = true;
        RestrictSUIDSGID = true;
        SystemCallArchitectures = "native";
        PrivateUsers = true;
        StateDirectory = "imagehost";
        ExecStart = "${lib.getExe cfg.package} -c=${tomlSetting}";
        Restart = "always";
      };

      environment.GIN_MODE = "release";
      wantedBy = [ "default.target" ];

      after = lib.mkIf (cfg.settings.database_type == "postgresql") [ "postgresql.service" ];
      requires = lib.mkIf (cfg.settings.database_type == "postgresql") [ "postgresql.service" ];
    };

    services.imagehost = lib.mkIf (cfg.createDbLocally && cfg.settings.database_type == "postgresql") {
      settings.database_connection_url = "postgresql:///imagehost?host=/run/postgresql&user=imagehost";
    };

    services.postgresql = lib.mkIf (cfg.createDbLocally && cfg.settings.database_type == "postgresql") {
      enable = true;
      ensureDatabases = [ "imagehost" ];
      ensureUsers = [
        {
          name = "imagehost";
          ensureDBOwnership = true;
        }
      ];
    };

    users.users.imagehost = {
      isSystemUser = true;
      group = "imagehost";
    };

    users.groups.imagehost = {};
  };
}
