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
              overlays = [
                nix-bun.overlays.default
                # Newer than nixpkgs; its tests need the network and a .git directory. Drop this
                # when nixpkgs has 2.6.0 or later.
                (final: prev: {
                  jactionlint = prev.jactionlint.overrideAttrs (old: rec {
                    version = "2.6.0";
                    src = final.fetchFromGitHub {
                      owner = "jdx";
                      repo = "jactionlint";
                      tag = "v${version}";
                      hash = "sha256-ikUVmurWrKP18XMth0lhnsvF0JT4gVoiNGTN+5AAL3U=";
                    };
                    vendorHash = "sha256-jf7hKPnaSl4iTYT0mtRRAEmgLKAA/lj0vSVoO9+iOvw=";
                    doCheck = false;
                  });
                })
              ];
            }
          )
        );
    in
    {
      devShells = forAll (pkgs: rec {
        # What every zeabur-oss repository's shell has: the release tools that @zeabur-oss/release
        # calls, knope for change files and release pull requests, and the linters of the shared CI (jactionlint checks workflows; actionlint stays until every
        # repository has moved to it). A repository's shell adds its own with
        # `inputsFrom = [ inputs.oss.devShells.${system}.base ];`.
        base = pkgs.mkShell {
          packages = with pkgs; [
            bun
            crane
            cosign
            gh
            kubernetes-helm
            actionlint
            jactionlint
            shellcheck
            jq
            nixfmt
            knope
          ];
        };
        default = base;
      });

    };
}
