{ buildGoModule }:
buildGoModule {
  src = ./.;

  name = "imagehost";
  vendorHash = "sha256-wYRKdWfC8q791vX/XBnTKNjoTQoekZXzAbYxVmWqH+8=";

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
