// Package sigstore signs and verifies with the cosign and gh commands.
//
// Signing is keyless, with the identity token of the GitHub Actions job: every signature names
// the workflow that ran. The commands are run rather than linked in so that a release signs and
// verifies with exactly the tools its users run, at the versions the dev shell pins.
package sigstore

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Issuer is the OIDC issuer of GitHub Actions' identity tokens.
const Issuer = "https://token.actions.githubusercontent.com"

// Identity is who must have signed: the shared release workflow, run for a repository at a tag.
type Identity struct {
	CertIdentityRegexp string // the certificate's identity, the signing workflow
	SignerWorkflow     string // the same workflow, as gh attestation verify --signer-workflow
	SourceRepository   string // the repository released, owner/name
	SourceRef          string // refs/tags/vX.Y.Z
}

// IdentityFromEnv reads the identity the shared release workflow passes to the verify stage
// (RELEASE_CERT_IDENTITY_REGEXP, RELEASE_SIGNER_WORKFLOW, RELEASE_SOURCE_REPOSITORY and
// RELEASE_SOURCE_REF). Unset, they are zeabur-oss/.github's release workflow, run for repository
// at the tag of version.
func IdentityFromEnv(repository, version string) (Identity, error) {
	id := Identity{
		CertIdentityRegexp: env("RELEASE_CERT_IDENTITY_REGEXP",
			`^https://github\.com/zeabur-oss/\.github/\.github/workflows/release\.yml@`),
		SignerWorkflow:   env("RELEASE_SIGNER_WORKFLOW", "zeabur-oss/.github/.github/workflows/release.yml"),
		SourceRepository: env("RELEASE_SOURCE_REPOSITORY", repository),
		SourceRef:        env("RELEASE_SOURCE_REF", "refs/tags/"+version),
	}
	if id.SourceRepository == "" {
		return id, fmt.Errorf("no repository: set RELEASE_SOURCE_REPOSITORY or pass --repository")
	}
	// A signature for another tag of the same repository is not this release's.
	if version != "" && id.SourceRef != "refs/tags/"+version {
		return id, fmt.Errorf("RELEASE_SOURCE_REF is %s, not the tag refs/tags/%s", id.SourceRef, version)
	}
	return id, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (id Identity) certificate() []string {
	return []string{
		"--certificate-oidc-issuer", Issuer,
		"--certificate-identity-regexp", id.CertIdentityRegexp,
		"--certificate-github-workflow-repository", id.SourceRepository,
		"--certificate-github-workflow-ref", id.SourceRef,
	}
}

// Runner runs a command. Tests replace it.
type Runner func(ctx context.Context, name string, args ...string) error

// Exec runs the command with this process's environment, its output on stderr so that stdout
// stays the command's result, and the command's own stderr in the error.
func Exec(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stdout = os.Stderr
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// Tools signs and verifies through cosign and gh.
type Tools struct{ Run Runner }

// SignImage signs an image or chart by digest (repository@sha256:...).
func (t Tools) SignImage(ctx context.Context, ref string) error {
	return t.Run(ctx, "cosign", "sign", "--yes", ref)
}

// SignBlob signs a file, writing the Sigstore bundle next to it.
func (t Tools) SignBlob(ctx context.Context, file, bundle string) error {
	return t.Run(ctx, "cosign", "sign-blob", "--yes", "--bundle", bundle, file)
}

// VerifyImage checks that the identity signed the image or chart.
func (t Tools) VerifyImage(ctx context.Context, id Identity, ref string) error {
	return t.Run(ctx, "cosign", append(append([]string{"verify"}, id.certificate()...), ref)...)
}

// VerifyBlob checks a file's Sigstore bundle against the identity.
func (t Tools) VerifyBlob(ctx context.Context, id Identity, file, bundle string) error {
	return t.Run(ctx, "cosign", append(append([]string{"verify-blob"}, id.certificate()...),
		"--bundle", bundle, file)...)
}

// VerifyProvenance checks the GitHub build provenance attestation of a file, or of an image or
// chart given as oci://repository@sha256:....
func (t Tools) VerifyProvenance(ctx context.Context, id Identity, subject string) error {
	return t.Run(ctx, "gh", "attestation", "verify", subject,
		"--repo", id.SourceRepository, "--signer-workflow", id.SignerWorkflow, "--source-ref", id.SourceRef)
}
