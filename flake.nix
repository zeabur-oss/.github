{
  description = "zeabur-oss shared CI and release";

  inputs = {
    nixpkgs.url = "https://flakehub.com/f/NixOS/nixpkgs/0.1"; # unstable Nixpkgs
    # Bun's own release binaries; Nixpkgs is behind, and bun fmt and bun check are newer.
    nix-bun = {
      url = "github:zeabur-oss/nix-bun";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      nix-bun,
    }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      forAll =
        f:
        nixpkgs.lib.genAttrs systems (
          system:
          f (
            import nixpkgs {
              inherit system;
              overlays = [ nix-bun.overlays.default ];
            }
          )
        );
    in
    {
      packages = forAll (pkgs: {
        # Run from a Rust repository's scripts/ci.sh. Cargo is the caller's: the report reads the
        # graph with the repository's pinned toolchain.
        zeabur-oss-unsafe-report = pkgs.writeShellApplication {
          name = "zeabur-oss-unsafe-report";
          runtimeInputs = with pkgs; [ git jq gawk gnugrep gnused coreutils ];
          text = builtins.readFile ./checks/unsafe-report.sh;
        };
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
