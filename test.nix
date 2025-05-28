{ nixosTest, nixosModule }:
nixosTest {
  name = "imagehost";
  nodes.machine = { ... }: {
    imports = [ nixosModule ];
    services.imagehost = {
      enable = true;
      createDbLocally = true;
      settings.database_type = "postgresql";
    };

    services.postgresql.enable = true;
  };

  testScript = { nodes, ... }: ''
    start_all()
    machine.wait_for_unit("postgresql.service")
    machine.wait_for_unit("imagehost.service")
    machine.wait_for_open_port(${toString nodes.machine.services.imagehost.settings.web_port})
  '';
}
