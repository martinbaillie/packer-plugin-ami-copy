{
  description = "packer-plugin-ami-copy";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          config.allowUnfree = true; # BSL2... Hashicorp...
        };
      in
      with pkgs;
      {
        devShells.default = mkShell {
          packages = [
            bashInteractive
            gnumake
            go
            goreleaser
            syft
            packer
          ];
        };
      }
    );
}
