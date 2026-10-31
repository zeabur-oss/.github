{
  description = "zeabur-oss shared CI and release";

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
        # calls, and the linters of the shared CI. A repository's shell adds its own with
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
          ];
        };
        default = base;
      });
    };
}
