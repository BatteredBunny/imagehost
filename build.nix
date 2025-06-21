{ buildGoModule }:
buildGoModule {
  src = ./.;

  name = "imagehost";
  vendorHash = "sha256-CBDm+fbinvNXBhimh5xMmv+w6HI6wv0vmOIgVVkgjoE=";

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
