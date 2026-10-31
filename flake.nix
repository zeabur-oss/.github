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
      # crane, cosign, gh and helm come with it: it pushes, signs and verifies through them.
      packages = forAll (pkgs: rec {
        # The tools go after the caller's PATH, as a fallback: the tests put stand-ins first.
        zeabur-oss-release = pkgs.writeShellScriptBin "zeabur-oss-release" ''
          export PATH="$PATH:${
            pkgs.lib.makeBinPath (
              with pkgs;
              [
                crane
                cosign
                gh
                kubernetes-helm
                coreutils
                gnutar
              ]
            )
          }"
          exec ${pkgs.lib.getExe pkgs.bun} ${./release/zeabur-oss-release.ts} "$@"
        '';
        # Run from a Rust repository's scripts/ci.sh. Cargo is the caller's: the report reads the
        # graph with the repository's pinned toolchain.
        zeabur-oss-unsafe-report = pkgs.writeShellApplication {
          name = "zeabur-oss-unsafe-report";
          runtimeInputs = with pkgs; [ git jq gawk gnugrep gnused coreutils ];
          text = builtins.readFile ./checks/unsafe-report.sh;
        };
        default = zeabur-oss-release;
      });

      devShells = forAll (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            bun
            crane
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
