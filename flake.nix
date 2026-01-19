{
  description = "Go Music DL - 一个完整的、工程化的 Go 音乐下载项目";

  inputs.nixpkgs-with-go_1_25.url = "github:NixOS/nixpkgs/nixos-unstable";
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  inputs.flake-utils.url = "github:numtide/flake-utils";
  inputs.gomod2nix.url = "github:nix-community/gomod2nix";
  inputs.gomod2nix.inputs.nixpkgs.follows = "nixpkgs";
  inputs.gomod2nix.inputs.flake-utils.follows = "flake-utils";

  outputs = {
    self,
    nixpkgs,
    nixpkgs-with-go_1_25,
    flake-utils,
    gomod2nix,
    ...
  } @ inputs: let
    allSystems = flake-utils.lib.allSystems;
  in (
    flake-utils.lib.eachSystem allSystems
    (system: let
      old-nixpkgs = nixpkgs-with-go_1_25.legacyPackages.${system};
      pkgs = import nixpkgs {
        inherit system;

        overlays = [
          (_: _: {
            go_1_25 = old-nixpkgs.go_1_25;
          })
        ];
      };

      # The current default sdk for macOS fails to compile go projects, so we use a newer one for now.
      # This has no effect on other platforms.
      callPackage = pkgs.darwin.apple_sdk_11_0.callPackage or pkgs.callPackage;
    in {
      # doCheck will fail at write files
      packages = rec {
        go-music-dl = (callPackage ./. (inputs
          // {
            inherit (gomod2nix.legacyPackages.${system}) buildGoApplication;
          }))
          .overrideAttrs (_: {doCheck = false;});

        default = go-music-dl;

        docker_builder = pkgs.dockerTools.buildLayeredImage {
          name = "go-music-dl";
          tag = "latest";
          contents = [
            self.packages.${system}.go-music-dl
            pkgs.cacert
          ];
        };
      };
      devShells.default = callPackage ./shell.nix {
        inherit (gomod2nix.legacyPackages.${system}) mkGoEnv gomod2nix;
      };
      formatter = pkgs.alejandra;
    })
  );
}
