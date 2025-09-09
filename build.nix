{ buildGoModule }:
buildGoModule {
  src = ./.;

  name = "imagehost";
  vendorHash = "sha256-3haVzVB7wpZJT40yFjlAHW/213PtLro+FudKGaYf/uI=";

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
