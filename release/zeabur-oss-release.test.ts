// Unit tests, then the commands against a registry in memory (crane registry serve) with images
// that `docker image save` wrote. cosign and gh are stand-ins that log their arguments: keyless
// signing needs a CI token. Run with `bun test release/` in the dev shell, with Docker.
import { $ } from "bun";
import { afterAll, beforeAll, describe, expect, test } from "bun:test";
import { mkdtemp, mkdir, rm, writeFile, chmod, cp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { Failure, Ref, parseDigests, recorded, section } from "./zeabur-oss-release";

const digest = (c: string) => `sha256:${c.repeat(64)}`;

describe("section", () => {
  const changelog = [
    "# Changelog",
    "",
    "## 0.2.0-rc.1 (2026-11-01)",
    "",
    "- A release candidate.",
    "",
    "## 0.1.1 (2026-10-20)",
    "",
    "### Fixes",
    "",
    "- One fix.",
    "",
  ].join("\n");

  test("is the lines under the version's heading", () => {
    expect(section(changelog, "v0.1.1")).toBe("### Fixes\n\n- One fix.");
  });

  test("of a pre-release is its own", () => {
    expect(section(changelog, "v0.2.0-rc.1")).toBe("- A release candidate.");
  });

  test("is refused when missing or empty", () => {
    expect(() => section(changelog, "v0.2.0")).toThrow(Failure);
    expect(() => section("## 0.3.0 (2026-12-01)\n\n## 0.2.0 (2026-11-01)\n- x\n", "v0.3.0")).toThrow(Failure);
  });
});

describe("digests.txt", () => {
  const text = [
    `builder linux/amd64  ghcr.io/zeabur-oss/builder@${digest("a")}`,
    `builder index  ghcr.io/zeabur-oss/builder@${digest("b")}`,
    `chart  ghcr.io/zeabur-oss/charts/zeabur-builder@${digest("c")}`,
    "",
  ].join("\n");

  test("names one reference per kind", () => {
    const entries = parseDigests(text);
    expect(recorded(entries, "builder index").toString()).toBe(`ghcr.io/zeabur-oss/builder@${digest("b")}`);
    expect(recorded(entries, "chart").repo).toBe("ghcr.io/zeabur-oss/charts/zeabur-builder");
  });

  test("refuses a kind that is missing or twice", () => {
    expect(() => recorded(parseDigests(text), "builder linux/arm64")).toThrow(Failure);
    expect(() => recorded(parseDigests(text + text), "chart")).toThrow(Failure);
  });

  test("refuses a reference that is not by digest", () => {
    expect(() => Ref.parse("ghcr.io/zeabur-oss/builder:0.1.1")).toThrow(Failure);
  });
});

describe("against a registry", () => {
  let work: string;
  let registry: ReturnType<typeof Bun.spawn>;
  let host: string;
  const cli = join(import.meta.dir, "zeabur-oss-release.ts");
  const env = () => ({
    ...process.env,
    PATH: `${join(work, "bin")}:${process.env.PATH}`,
    CALLS: join(work, "calls.log"),
    RELEASE_CERT_IDENTITY_REGEXP: "^https://github\\.com/zeabur-oss/\\.github/\\.github/workflows/release\\.yml@",
    RELEASE_SIGNER_WORKFLOW: "zeabur-oss/.github/.github/workflows/release.yml",
    RELEASE_SOURCE_REPOSITORY: "zeabur-oss/smoke",
    RELEASE_SOURCE_REF: "refs/tags/v0.0.1",
  });
  const run = (args: string[]) => $`bun ${cli} ${args}`.env(env()).nothrow().quiet();
  const calls = async () => (await Bun.file(join(work, "calls.log")).text()).trim().split("\n");

  beforeAll(async () => {
    work = await mkdtemp(join(tmpdir(), "release-test-"));
    await mkdir(join(work, "bin"));
    for (const tool of ["cosign", "gh"]) {
      const stub = join(work, "bin", tool);
      // sign-blob writes its bundle; everything else succeeds.
      await writeFile(
        stub,
        `#!/usr/bin/env bash\necho "${tool} $*" >>"$CALLS"\n` +
          `while [[ $# -gt 0 ]]; do [[ "$1" == --bundle && "${tool}" == cosign ]] && echo '{}' >"$2"; shift; done\n`,
      );
      await chmod(stub, 0o755);
    }
    const port = 5100 + Math.floor(Math.random() * 800);
    host = `localhost:${port}`;
    registry = Bun.spawn(["crane", "registry", "serve", "--address", host], { stderr: "ignore" });
    for (let i = 0; i < 50 && (await $`curl -fs http://${host}/v2/`.nothrow().quiet()).exitCode !== 0; i++)
      await Bun.sleep(100);
    for (const arch of ["amd64", "arm64"]) {
      await $`printf 'FROM scratch\nCOPY zeabur-oss-release.ts /\n' | docker build -q --platform linux/${arch} --tag release-test:${arch} --file - .`
        .cwd(import.meta.dir)
        .quiet();
      await $`docker image save --output ${join(work, `${arch}.tar`)} release-test:${arch}`;
    }
    await $`helm create ${join(work, "smoke")}`.quiet();
    await $`sed -i.bak -e 's/^version: .*/version: 0.0.1/' ${join(work, "smoke", "Chart.yaml")}`;
    await $`helm package ${join(work, "smoke")} --destination ${work}`.quiet();
  });

  afterAll(async () => {
    registry?.kill();
    await rm(work, { recursive: true, force: true });
  });

  test("push-image pushes by digest and tags only the index", async () => {
    const out = join(work, "published");
    await mkdir(out);
    const pushed = await run([
      "push-image", out, "smoke", `${host}/zeabur-oss/smoke:0.0.1`,
      `linux/amd64=${join(work, "amd64.tar")}`, `linux/arm64=${join(work, "arm64.tar")}`,
    ]);
    expect(pushed.stderr.toString()).toBe("");
    expect(pushed.exitCode).toBe(0);

    const entries = parseDigests(await Bun.file(join(out, "digests.txt")).text());
    const index = recorded(entries, "smoke index");
    const listed = (await $`crane manifest ${index}`.json()).manifests
      .map((m: any) => `${m.platform.os}/${m.platform.architecture} ${m.digest}`)
      .sort();
    expect(listed).toEqual([
      `linux/amd64 ${recorded(entries, "smoke linux/amd64").digest}`,
      `linux/arm64 ${recorded(entries, "smoke linux/arm64").digest}`,
    ]);
    expect((await $`crane ls ${host}/zeabur-oss/smoke`.text()).trim()).toBe("0.0.1");
    expect(await Bun.file(join(out, "subjects.json")).json()).toEqual([
      { name: `${host}/zeabur-oss/smoke`, digest: index.digest },
    ]);
    const signed = (await calls()).filter((c) => c.startsWith("cosign sign --yes"));
    expect(signed.sort()).toEqual(entries.map((e) => `cosign sign --yes ${e.ref}`).sort());
  });

  test("push-image refuses a version that exists", async () => {
    const again = await run([
      "push-image", join(work, "published"), "smoke", `${host}/zeabur-oss/smoke:0.0.1`,
      `linux/amd64=${join(work, "amd64.tar")}`,
    ]);
    expect(again.exitCode).toBe(1);
    expect(again.stderr.toString()).toContain("already exists");
  });

  test("push-image refuses an image for another platform", async () => {
    const wrong = await run([
      "push-image", join(work, "published"), "smoke", `${host}/zeabur-oss/smoke:0.0.2`,
      `linux/arm64=${join(work, "amd64.tar")}`,
    ]);
    expect(wrong.exitCode).toBe(1);
    expect(wrong.stderr.toString()).toContain("is not a linux/arm64 image");
  });

  test("verify-image accepts what was pushed and refuses swapped digests", async () => {
    const digests = join(work, "published", "digests.txt");
    const ok = await run(["verify-image", digests, "smoke", "linux/amd64", "linux/arm64"]);
    expect(ok.stderr.toString()).toBe("");
    expect(ok.exitCode).toBe(0);

    const swapped = join(work, "swapped.txt");
    const text = await Bun.file(digests).text();
    await writeFile(swapped, text.replace("linux/amd64", "linux/TMP").replace("linux/arm64", "linux/amd64").replace("linux/TMP", "linux/arm64"));
    const bad = await run(["verify-image", swapped, "smoke", "linux/amd64", "linux/arm64"]);
    expect(bad.exitCode).toBe(1);
    expect(bad.stderr.toString()).toContain("the index");
  });

  test("push-chart pushes, checks and signs; verify-chart reads it back", async () => {
    const out = join(work, "published");
    const chart = join(work, "smoke-0.0.1.tgz");
    const pushed = await run(["push-chart", out, `oci://${host}/zeabur-oss/charts`, chart]);
    expect(pushed.exitCode).toBe(0);
    const ref = recorded(parseDigests(await Bun.file(join(out, "digests.txt")).text()), "chart");
    expect(ref.repo).toBe(`${host}/zeabur-oss/charts/smoke`);
    expect(await calls()).toContain(`cosign sign --yes ${ref}`);

    expect((await run(["verify-chart", join(out, "digests.txt"), chart])).exitCode).toBe(0);
    const other = join(work, "other.tgz");
    await writeFile(other, "not the chart");
    const bad = await run(["verify-chart", join(out, "digests.txt"), other]);
    expect(bad.exitCode).toBe(1);
    expect(bad.stderr.toString()).toContain("is not");
  });

  test("checksums and verify-assets cover exactly the assets", async () => {
    const assets = join(work, "assets");
    await mkdir(assets);
    await cp(join(work, "published", "digests.txt"), join(assets, "digests.txt"));
    await cp(join(work, "smoke-0.0.1.tgz"), join(assets, "smoke-0.0.1.tgz"));
    expect((await run(["checksums", assets])).exitCode).toBe(0);
    expect((await Bun.file(join(assets, "SHA256SUMS")).text()).split("\n").filter(Boolean).map((l) => l.split(/\s+/)[1]))
      .toEqual(["digests.txt", "smoke-0.0.1.tgz"]);
    expect((await run(["verify-assets", assets])).exitCode).toBe(0);

    await writeFile(join(assets, "extra.txt"), "extra");
    const extra = await run(["verify-assets", assets]);
    expect(extra.exitCode).toBe(1);
    expect(extra.stderr.toString()).toContain("not the assets");
  });
});
