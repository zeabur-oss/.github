{
  description = "zeabur-oss shared CI and release: zeabur-oss-release and its dev shell";

  inputs = {
    nixpkgs.url = "https://flakehub.com/f/NixOS/nixpkgs/0.1"; # unstable Nixpkgs
  };

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      forAll = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      # A repository adds this to its dev shell, pinned by its flake.lock:
      #   inputs.oss-release.url = "github:zeabur-oss/.github";
      #   packages = [ oss-release.packages.${system}.zeabur-oss-release ];
      # cosign, gh and helm come with it: it signs, verifies and pushes charts through them.
      packages = forAll (pkgs: rec {
        zeabur-oss-release = pkgs.buildGoModule {
          pname = "zeabur-oss-release";
          version = "0";
          src = ./release;
          subPackages = [ "cmd/zeabur-oss-release" ];
          # Change it when go.sum changes: nix build prints the new value.
          vendorHash = "sha256-DLnStkxCwp8yPpkYMINy3ZsDL0l6Wy7AumoBrqRjae0=";
          env.CGO_ENABLED = 0;
          ldflags = [ "-s" "-w" ];
          nativeBuildInputs = [ pkgs.makeWrapper ];
          postInstall = ''
            wrapProgram $out/bin/zeabur-oss-release \
              --suffix PATH : ${pkgs.lib.makeBinPath [ pkgs.cosign pkgs.gh pkgs.kubernetes-helm ]}
          '';
        };
        default = zeabur-oss-release;
      });

      devShells = forAll (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            cosign
            gh
            kubernetes-helm
            actionlint
            shellcheck
            jq
          ];
        };
      });
    };
}
