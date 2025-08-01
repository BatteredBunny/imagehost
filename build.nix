{ buildGoModule }:
buildGoModule {
  src = ./.;

  name = "imagehost";
  vendorHash = "sha256-/X9R+CfpluclX+sH81sWOXpSgCMjGMuptLi657BlVNI=";

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
