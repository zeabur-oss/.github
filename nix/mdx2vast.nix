# Vale lints .mdx through this converter; nixpkgs does not have it.
{
  buildNpmPackage,
  fetchFromGitHub,
  lib,
}:

buildNpmPackage rec {
  pname = "mdx2vast";
  version = "0.5.0";

  src = fetchFromGitHub {
    owner = "jdkato";
    repo = "mdx2vast";
    tag = "v${version}";
    hash = "sha256-rAaZ6V/U7wjZWYr7N+KNu96PX+U0ZyUWZWiaJWrukWE=";
  };

  # Change it when the version changes: nix build prints the new value.
  npmDepsHash = "sha256-EE2EtZmZFt+ynSNT1zQmbwZaA+mcxV/0Ck51t0wsyrA=";
  dontNpmBuild = true;

  meta = {
    description = "Converts MDX to the HTML Vale lints";
    homepage = "https://github.com/jdkato/mdx2vast";
    license = lib.licenses.mit;
    mainProgram = "mdx2vast";
  };
}
