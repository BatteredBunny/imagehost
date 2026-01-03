{ buildGoModule }:
buildGoModule {
  src = ./.;

  name = "hostling";
  version = "0.2.0";
  vendorHash = "sha256-sNQH/mKPEmeU1OVT7SadIyDGB9p57GpefYVMN804U8Y=";

  ldflags = [
    "-s"
    "-w"
  ];

  meta = {
    description = "Simple file hosting service";
    homepage = "https://github.com/BatteredBunny/hostling";
    mainProgram = "hostling";
  };
}
