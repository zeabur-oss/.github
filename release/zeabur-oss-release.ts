// The steps of a release that every zeabur-oss repository does the same way, on crane, helm,
// cosign and gh. A repository's scripts/release.ts calls it for these and keeps the rest.
//
// push-* append to <out>/digests.txt and <out>/subjects.json. digests.txt has one line per image,
// index or chart: its kind (`<label> <os>/<arch>`, `<label> index` or `chart`; an empty label is
// left out), two spaces and the reference by digest. subjects.json is what the workflow attests.
// verify-* read the signer from the RELEASE_* variables the shared release workflow sets.
import { $ } from "bun";
import { appendFile, readdir, rm } from "node:fs/promises";
import { join } from "node:path";

const usage = `usage:
  zeabur-oss-release absent <ref>...
  zeabur-oss-release push-image <out> <label> <repo>:<tag> <os>/<arch>=<image.tar>...
  zeabur-oss-release push-chart <out> <oci-repo> <chart.tgz>
  zeabur-oss-release checksums <dir>
  zeabur-oss-release notes <CHANGELOG.md> <vX.Y.Z> [<docs-url>]
  zeabur-oss-release verify-assets <dir>
  zeabur-oss-release verify-image <digests.txt> <label> <os>/<arch>...
  zeabur-oss-release verify-chart <digests.txt> <chart.tgz>`;

export class Failure extends Error {}

function fail(message: string): never {
  throw new Failure(message);
}

// A reference by digest: ghcr.io/zeabur-oss/builder@sha256:...
export class Ref {
  constructor(
    readonly repo: string,
    readonly digest: string,
  ) {}

  static parse(text: string): Ref {
    const m = /^([^\s@]+)@(sha256:[0-9a-f]{64})$/.exec(text);
    if (!m) fail(`${text} is not a reference by digest`);
    return new Ref(m[1], m[2]);
  }

  at(digest: string): Ref {
    return new Ref(this.repo, digest);
  }

  toString(): string {
    return `${this.repo}@${this.digest}`;
  }
}

export function kind(label: string, what: string): string {
  return label ? `${label} ${what}` : what;
}

export function parseDigests(text: string): { kind: string; ref: Ref }[] {
  return text
    .split("\n")
    .filter((line) => line.trim())
    .map((line) => {
      const at = line.lastIndexOf(" ");
      return { kind: line.slice(0, at).trim(), ref: Ref.parse(line.slice(at + 1)) };
    });
}

// The one reference digests.txt records for a kind.
export function recorded(entries: { kind: string; ref: Ref }[], what: string): Ref {
  const found = entries.filter((e) => e.kind === what);
  if (found.length !== 1) fail(`digests.txt has ${found.length} entries for ${what}, not one`);
  return found[0].ref;
}

// The lines under `## X.Y.Z (date)`, as they are. CHANGELOG.md keeps each paragraph and list item
// on one line, since a release body shows every line break.
export function section(changelog: string, version: string): string {
  const plain = version.replace(/^v/, "");
  const lines = changelog.split("\n");
  const start = lines.findIndex((l) => l.startsWith("## ") && l.split(" ")[1] === plain);
  if (start < 0) fail(`CHANGELOG.md has no section for ${plain}`);
  const end = lines.findIndex((l, i) => i > start && l.startsWith("## "));
  const body = lines.slice(start + 1, end < 0 ? undefined : end).join("\n").trim();
  if (!body) fail(`CHANGELOG.md's section for ${plain} is empty`);
  return body;
}

// Records what was pushed, in <out>/digests.txt and <out>/subjects.json.
class Published {
  constructor(readonly dir: string) {}

  async record(what: string, ref: Ref, attest = false) {
    await appendFile(join(this.dir, "digests.txt"), `${what}  ${ref}\n`);
    const file = Bun.file(join(this.dir, "subjects.json"));
    const subjects = (await file.exists()) ? await file.json() : [];
    if (attest) subjects.push({ name: ref.repo, digest: ref.digest });
    await Bun.write(file, JSON.stringify(subjects) + "\n");
  }
}

// Fails unless the registry answers that the reference does not exist.
async function absent(ref: string) {
  const { exitCode, stderr } = await $`crane digest ${ref}`.nothrow().quiet();
  if (exitCode === 0) fail(`${ref} already exists; inspect it before trying again`);
  // The tag is not there, or the repository is not there yet.
  if (!/MANIFEST_UNKNOWN|NAME_UNKNOWN/.test(stderr.toString())) fail(`cannot check ${ref}: ${stderr}`);
}

// The platform of a `docker image save` archive, from its image config.
async function platform(tar: string): Promise<string> {
  const manifest = await $`tar -xOf ${tar} manifest.json`.json();
  if (manifest.length !== 1) fail(`${tar} holds ${manifest.length} images, not one`);
  const config = await $`tar -xOf ${tar} ${manifest[0].Config}`.json();
  return `${config.os}/${config.architecture}`;
}

// The chart layer of the chart in the registry is the packaged file.
async function chartIs(ref: Ref, chart: string) {
  const manifest = await $`crane manifest ${ref}`.json();
  const layers = manifest.layers.filter(
    (l: { mediaType: string }) => l.mediaType === "application/vnd.cncf.helm.chart.content.v1.tar+gzip",
  );
  const digest = `sha256:${new Bun.CryptoHasher("sha256").update(await Bun.file(chart).bytes()).digest("hex")}`;
  if (layers.length !== 1 || layers[0].digest !== digest) fail(`${ref} is not ${chart}`);
}

// Who must have signed: the shared release workflow, run for this repository at the tag.
function signer() {
  const env = (name: string) => process.env[name] || fail(`${name} is not set`);
  const repository = env("RELEASE_SOURCE_REPOSITORY");
  const ref = env("RELEASE_SOURCE_REF");
  return {
    cosign: [
      "--certificate-oidc-issuer", "https://token.actions.githubusercontent.com",
      "--certificate-identity-regexp", env("RELEASE_CERT_IDENTITY_REGEXP"),
      "--certificate-github-workflow-repository", repository,
      "--certificate-github-workflow-ref", ref,
    ],
    gh: ["--repo", repository, "--signer-workflow", env("RELEASE_SIGNER_WORKFLOW"), "--source-ref", ref],
  };
}

async function pushImage(out: string, label: string, tagged: string, archives: string[]) {
  const repo = tagged.slice(0, tagged.lastIndexOf(":"));
  const published = new Published(out);
  await absent(tagged);
  // Each platform image by its digest alone; the index that lists them is pushed last, with the
  // tag, so a run that fails before it leaves no tag and can be repeated.
  const manifests: string[] = [];
  for (const archive of archives) {
    const [want, tar] = archive.split("=", 2);
    if ((await platform(tar)) !== want) fail(`${tar} is not a ${want} image`);
    const ref = new Ref(repo, (await $`crane digest --tarball ${tar}`.text()).trim());
    await $`crane push ${tar} ${ref}`.quiet();
    await $`cosign sign --yes ${ref}`;
    await published.record(kind(label, want), ref);
    manifests.push("--manifest", ref.toString());
  }
  const index = Ref.parse((await $`crane index append --docker-empty-base ${manifests} --tag ${tagged}`.text()).trim());
  await $`cosign sign --yes ${index}`;
  await published.record(kind(label, "index"), index, true);
}

async function pushChart(out: string, ociRepo: string, chart: string) {
  const repo = ociRepo.replace(/^oci:\/\//, "");
  const meta = Bun.YAML.parse(await $`helm show chart ${chart}`.text()) as { name: string; version: string };
  const tagged = `${repo}/${meta.name}:${meta.version}`;
  await absent(tagged);
  // crane talks plain HTTP to localhost by itself; helm needs to be told.
  const plainHttp = repo.startsWith("localhost:") ? ["--plain-http"] : [];
  await $`helm push ${chart} oci://${repo} ${plainHttp}`;
  const ref = new Ref(`${repo}/${meta.name}`, (await $`crane digest ${tagged}`.text()).trim());
  await chartIs(ref, chart);
  await $`cosign sign --yes ${ref}`;
  await new Published(out).record("chart", ref, true);
}

async function checksums(dir: string) {
  await rm(join(dir, "SHA256SUMS"), { force: true });
  await rm(join(dir, "SHA256SUMS.sigstore.json"), { force: true });
  const files = (await readdir(dir)).sort();
  const sums = await $`sha256sum -- ${files}`.cwd(dir).text();
  await Bun.write(join(dir, "SHA256SUMS"), sums);
  await $`cosign sign-blob --yes --bundle ${join(dir, "SHA256SUMS.sigstore.json")} ${join(dir, "SHA256SUMS")}`;
}

async function notes(changelog: string, version: string, docs?: string) {
  let text = section(await Bun.file(changelog).text(), version) + "\n";
  if (docs) text += `\nInstall, upgrade and verify: [${docs.replace(/^https:\/\//, "").replace(/\/$/, "")}](${docs})\n`;
  process.stdout.write(text);
}

async function verifyAssets(dir: string) {
  const id = signer();
  await $`cosign verify-blob ${id.cosign} --bundle ${join(dir, "SHA256SUMS.sigstore.json")} ${join(dir, "SHA256SUMS")}`;
  // SHA256SUMS lists every other asset, and nothing else.
  const listed = (await Bun.file(join(dir, "SHA256SUMS")).text()).trim().split("\n").map((l) => l.split(/\s+/)[1]).sort();
  const assets = (await readdir(dir)).filter((f) => f !== "SHA256SUMS" && f !== "SHA256SUMS.sigstore.json").sort();
  if (listed.join(" ") !== assets.join(" ")) fail(`SHA256SUMS lists ${listed.join(" ")}, not the assets ${assets.join(" ")}`);
  await $`sha256sum --check --strict SHA256SUMS`.cwd(dir);
  for (const asset of assets) await $`gh attestation verify ${join(dir, asset)} ${id.gh}`;
}

async function verifyImage(digestsFile: string, label: string, platforms: string[]) {
  const id = signer();
  const entries = parseDigests(await Bun.file(digestsFile).text());
  const index = recorded(entries, kind(label, "index"));
  // The index holds exactly the recorded platform images, each signed.
  const want = platforms.map((p) => {
    const ref = recorded(entries, kind(label, p));
    if (ref.repo !== index.repo) fail(`${ref} is not in ${index.repo}`);
    return `${p} ${ref.digest}`;
  });
  const manifest = await $`crane manifest ${index}`.json();
  const got = manifest.manifests.map(
    (m: { platform: { os: string; architecture: string }; digest: string }) =>
      `${m.platform.os}/${m.platform.architecture} ${m.digest}`,
  );
  if (got.sort().join(", ") !== want.sort().join(", "))
    fail(`the index ${index} holds ${got.join(", ")}, not ${want.join(", ")}`);
  for (const ref of [index, ...want.map((w) => index.at(w.split(" ")[1]))])
    await $`cosign verify ${id.cosign} ${ref}`.quiet();
  await $`gh attestation verify oci://${index} ${id.gh}`;
  console.log(`verified ${index}`);
}

async function verifyChart(digestsFile: string, chart: string) {
  const id = signer();
  const ref = recorded(parseDigests(await Bun.file(digestsFile).text()), "chart");
  await chartIs(ref, chart);
  await $`cosign verify ${id.cosign} ${ref}`.quiet();
  await $`gh attestation verify oci://${ref} ${id.gh}`;
  console.log(`verified ${ref}`);
}

async function main(args: string[]) {
  const [command, ...rest] = args;
  const need = (ok: boolean) => ok || fail(usage);
  switch (command) {
    case "absent":
      need(rest.length > 0);
      for (const ref of rest) await absent(ref);
      return;
    case "push-image":
      need(rest.length >= 4);
      return pushImage(rest[0], rest[1], rest[2], rest.slice(3));
    case "push-chart":
      need(rest.length === 3);
      return pushChart(rest[0], rest[1], rest[2]);
    case "checksums":
      need(rest.length === 1);
      return checksums(rest[0]);
    case "notes":
      need(rest.length === 2 || rest.length === 3);
      return notes(rest[0], rest[1], rest[2]);
    case "verify-assets":
      need(rest.length === 1);
      return verifyAssets(rest[0]);
    case "verify-image":
      need(rest.length >= 3);
      return verifyImage(rest[0], rest[1], rest.slice(2));
    case "verify-chart":
      need(rest.length === 2);
      return verifyChart(rest[0], rest[1]);
    default:
      fail(usage);
  }
}

if (import.meta.main) {
  try {
    await main(process.argv.slice(2));
  } catch (e) {
    if (e instanceof Failure) console.error(`zeabur-oss-release: ${e.message}`);
    else if (e instanceof $.ShellError) console.error(`zeabur-oss-release: ${e.message}\n${e.stderr}`);
    else throw e;
    process.exit(1);
  }
}
