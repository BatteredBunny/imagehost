{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self
    , nixpkgs
    , ...
    }:
    let
      inherit (nixpkgs) lib;

      systems = lib.systems.flakeExposed;

      forAllSystems = lib.genAttrs systems;

      nixpkgsFor = forAllSystems (system: import nixpkgs {
        inherit system;
      });
    in
    {
      overlays.default = final: prev: {
        imagehost = final.callPackage ./build.nix { };
      };

      nixosModules.default = import ./module.nix;

      checks = forAllSystems (system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          service = pkgs.callPackage ./test.nix { nixosModule = self.nixosModules.default; };
        }
      );

      packages = forAllSystems (system:
        let
          pkgs = nixpkgsFor.${system};
          overlay = lib.makeScope pkgs.newScope (final: self.overlays.default final pkgs);
        in
        {
          inherit (overlay) imagehost;
          default = overlay.imagehost;
        }
      );

      devShells = forAllSystems (system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          default = pkgs.mkShell {
            buildInputs = with pkgs; [
              go
              wire
              sqlite
            ];
          };
        });
    };
}
