{
  description = "Nix build for the libfreerdp Go proof of concept";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
        go = pkgs.go_1_25;
        freerdp = pkgs.freerdp.overrideAttrs (old: {
          cmakeFlags = (old.cmakeFlags or [ ]) ++ [
            "-DWITH_OPENH264=OFF"
            "-DWITH_OPENH264_LOADING=OFF"
            "-DWITH_FFMPEG=ON"
          ];
        });
      in
      {
        packages.default = (pkgs.buildGoModule.override { go = go; }) {
          pname = "libfreerdp-golang-poc";
          version = "0.1.0";

          src = ./.;
          modRoot = ".";
          subPackages = [ "cmd/server" ];

          vendorHash = "sha256-icTfrV7YRkUOVqbDnWy+tb8q4GU7NwPK0cY96Y0RdPg=";

          nativeBuildInputs = [
            pkgs.pkg-config
          ];

          buildInputs = [
            freerdp
          ];

          env.CGO_ENABLED = "1";

          ldflags = [
            "-s"
            "-w"
          ];

          meta = with pkgs.lib; {
            description = "Headless FreeRDP-backed screenshot and control server written in Go";
            license = licenses.mit;
            mainProgram = "server";
            platforms = platforms.linux;
          };
        };

        apps.default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/server";
        };

        devShells.default = pkgs.mkShell {
          packages = [
            go
            pkgs.gopls
            pkgs.pkg-config
            freerdp
          ];

          env.CGO_ENABLED = "1";
        };
      }
    );
}
