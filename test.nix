{ testers }: let
  port = 8080;
 in
testers.nixosTest {
  name = "imagehost";

  interactive.nodes.machine = {
    services.imagehost.openFirewall = true;

    virtualisation.forwardPorts = [
      {
        from = "host";
        host.port = 8080;
        guest.port = port;
      }
    ];
  };

  nodes.machine = { ... }: {
    imports = [ ./module.nix ];
    services.imagehost = {
      enable = true;
      createDbLocally = true;
      settings.database_type = "postgresql";
      settings.port = port;
    };

    services.postgresql.enable = true;
  };

  testScript = { nodes, ... }: ''
    start_all()
    machine.wait_for_unit("postgresql.service")
    machine.wait_for_unit("imagehost.service")
    machine.wait_for_open_port(${toString nodes.machine.services.imagehost.settings.port})
  '';
}
