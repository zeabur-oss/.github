{
  description = "zeabur-oss shared CI, release and agent skills";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
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
      devShells = forAll (pkgs: rec {
        # What every zeabur-oss repository's shell has: the release tools that @zeabur-oss/release
        # calls, knope for change files and release pull requests, and the linters of the shared CI. A repository's shell adds its own with
        # `inputsFrom = [ inputs.oss.devShells.${system}.base ];`.
        base = pkgs.mkShell {
          packages = with pkgs; [
            bun
            crane
            cosign
            gh
            kubernetes-helm
            actionlint
            shellcheck
            jq
            nixfmt
            knope
          ];
        };
        # This repository's own checks: the shared skills and their Vale style.
        default = pkgs.mkShell {
          inputsFrom = [ base ];
          packages = [
            pkgs.vale
            pkgs.yq-go
            self.packages.${pkgs.stdenv.hostPlatform.system}.mdx2vast
          ];
        };
      });

      # Vale lints .mdx through mdx2vast. A repository that checks its text with the zeabur-writing
      # style adds `pkgs.vale` and `inputs.oss.packages.${system}.mdx2vast` to its dev shell.
      packages = forAll (pkgs: {
        mdx2vast = pkgs.callPackage ./nix/mdx2vast.nix { };
      });
    };
}
