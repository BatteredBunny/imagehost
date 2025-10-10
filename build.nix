{ buildGoModule }:
buildGoModule {
  src = ./.;

  name = "imagehost";
  vendorHash = "sha256-GqyyN5DWXjs2RYQhyGXTkWCajYRMr3Dqwi9BXgfYYCE=";

  ldflags = [
    "-s"
    "-w"
  ];

  meta = {
    description = "Simple imagehost written in Go";
    homepage = "https://github.com/BatteredBunny/imagehost";
    mainProgram = "imagehost";
  };
}
