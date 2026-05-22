{
  description = "Nix build for headlessdesk";

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
          pname = "headlessdesk";
          version = "0.1.0";

          src = ./.;
          modRoot = ".";
          subPackages = [ "cmd/headlessdesk" ];

          vendorHash = "sha256-om6zBU65ZwPGqk921ku0P5hFnX4vpQnjLzS6euEo4HM=";

          nativeBuildInputs = [
            pkgs.makeWrapper
            pkgs.pkg-config
          ];

          buildInputs = [
            freerdp
            pkgs.libvncserver
          ];

          env.CGO_ENABLED = "1";

          ldflags = [
            "-s"
            "-w"
          ];

          postInstall = ''
            wrapProgram $out/bin/headlessdesk \
              --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.fuse3 ]}
          '';

          meta = with pkgs.lib; {
            description = "Headless remote desktop screenshot and control server written in Go";
            license = licenses.gpl3Plus;
            mainProgram = "headlessdesk";
            platforms = platforms.linux;
          };
        };

        apps.default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/headlessdesk";
        };

        devShells.default = pkgs.mkShell {
          packages = [
            go
            pkgs.gopls
            pkgs.pkg-config
            freerdp
            pkgs.fuse3
            pkgs.libvncserver
          ];

          env.CGO_ENABLED = "1";
        };
      }
    );
}
