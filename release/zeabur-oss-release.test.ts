// Unit tests, then the functions against a registry in memory (crane registry serve) with images
// that `docker image save` wrote, and the command. cosign and gh are stand-ins that log their arguments: keyless
// signing needs a CI token. Run with `bun test release/` in the dev shell, with Docker.
import { $ } from "bun";
import { afterAll, beforeAll, describe, expect, test } from "bun:test";
import { mkdtemp, mkdir, rm, writeFile, chmod, cp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  Failure,
  Ref,
  checksums,
  listedIn,
  parseDigests,
  pushChart,
  pushImage,
  readDigests,
  recorded,
  section,
  verifyAssets,
  verifyChart,
  verifyImage,
} from "./zeabur-oss-release";

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
    expect(() => section(changelog, "v0.1.")).toThrow(Failure);
    expect(() =>
      section(
        "## 0.3.0 (2026-12-01)\n\n## 0.2.0 (2026-11-01)\n- x\n",
        "v0.3.0",
      ),
    ).toThrow(Failure);
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
    expect(recorded(entries, "builder index").toString()).toBe(
      `ghcr.io/zeabur-oss/builder@${digest("b")}`,
    );
    expect(recorded(entries, "chart").repo).toBe(
      "ghcr.io/zeabur-oss/charts/zeabur-builder",
    );
  });

  test("refuses a kind that is missing or twice", () => {
    expect(() => recorded(parseDigests(text), "builder linux/arm64")).toThrow(
      Failure,
    );
    expect(() => recorded(parseDigests(text + text), "chart")).toThrow(Failure);
  });

  test("refuses a reference that is not by digest", () => {
    expect(() => Ref.parse("ghcr.io/zeabur-oss/builder:0.1.1")).toThrow(
      Failure,
    );
  });
});

describe("SHA256SUMS", () => {
  const sum = "a".repeat(64);

  test("lists each file name whole", () => {
    expect(listedIn(`${sum}  My App.dmg\n${sum} *digests.txt\n`)).toEqual([
      "My App.dmg",
      "digests.txt",
    ]);
  });

  test("refuses a line that is not a checksum", () => {
    expect(() => listedIn("My App.dmg\n")).toThrow(Failure);
  });
});

describe("against a registry", () => {
  let work: string;
  let registry: ReturnType<typeof Bun.spawn>;
  let host: string;
  let saved: NodeJS.ProcessEnv;
  const calls = async () =>
    (await Bun.file(join(work, "calls.log")).text()).trim().split("\n");
  const images = () => ({
    "linux/amd64": join(work, "amd64.tar"),
    "linux/arm64": join(work, "arm64.tar"),
  });

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
    saved = { ...process.env };
    Object.assign(process.env, {
      PATH: `${join(work, "bin")}:${process.env.PATH}`,
      CALLS: join(work, "calls.log"),
      RELEASE_CERT_IDENTITY_REGEXP:
        "^https://github\\.com/zeabur-oss/\\.github/\\.github/workflows/release\\.yml@",
      RELEASE_SIGNER_WORKFLOW:
        "zeabur-oss/.github/.github/workflows/release.yml",
      RELEASE_SOURCE_REPOSITORY: "zeabur-oss/smoke",
      RELEASE_SOURCE_REF: "refs/tags/v0.0.1",
    });
    const port = 5100 + Math.floor(Math.random() * 800);
    host = `localhost:${port}`;
    registry = Bun.spawn(["crane", "registry", "serve", "--address", host], {
      stderr: "ignore",
    });
    for (
      let i = 0;
      i < 50 &&
      (await $`curl -fs http://${host}/v2/`.nothrow().quiet()).exitCode !== 0;
      i++
    )
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
    await mkdir(join(work, "published"));
    // docker build and helm on a cold runner take longer than a test.
  }, 120_000);

  afterAll(async () => {
    registry?.kill();
    process.env = saved;
    await rm(work, { recursive: true, force: true });
  });

  test("pushImage pushes by digest and tags only the index", async () => {
    const out = join(work, "published");
    const index = await pushImage(
      out,
      "smoke",
      `${host}/zeabur-oss/smoke:0.0.1`,
      images(),
    );

    const entries = await readDigests(join(out, "digests.txt"));
    expect(recorded(entries, "smoke index")).toEqual(index);
    const listed = (await $`crane manifest ${index}`.json()).manifests
      .map(
        (m: any) => `${m.platform.os}/${m.platform.architecture} ${m.digest}`,
      )
      .sort();
    expect(listed).toEqual([
      `linux/amd64 ${recorded(entries, "smoke linux/amd64").digest}`,
      `linux/arm64 ${recorded(entries, "smoke linux/arm64").digest}`,
    ]);
    expect((await $`crane ls ${host}/zeabur-oss/smoke`.text()).trim()).toBe(
      "0.0.1",
    );
    expect(await Bun.file(join(out, "subjects.json")).json()).toEqual([
      { name: `${host}/zeabur-oss/smoke`, digest: index.digest },
    ]);
    const signed = (await calls()).filter((c) =>
      c.startsWith("cosign sign --yes"),
    );
    expect(signed.sort()).toEqual(
      entries.map((e) => `cosign sign --yes ${e.ref}`).sort(),
    );
  });

  test("pushImage refuses a version that exists", async () => {
    const again = pushImage(
      join(work, "published"),
      "smoke",
      `${host}/zeabur-oss/smoke:0.0.1`,
      images(),
    );
    await expect(again).rejects.toThrow("already exists");
  });

  test("pushImage refuses an image for another platform before it pushes or records any", async () => {
    const out = join(work, "wrong");
    await mkdir(out);
    const before = await calls();
    const wrong = pushImage(out, "smoke", `${host}/zeabur-oss/smoke:0.0.2`, {
      "linux/amd64": join(work, "amd64.tar"),
      "linux/arm64": join(work, "amd64.tar"),
    });
    await expect(wrong).rejects.toThrow("is not a linux/arm64 image");
    expect(await Bun.file(join(out, "digests.txt")).exists()).toBe(false);
    expect(await calls()).toEqual(before);
  });

  test("verifyImage accepts what was pushed and refuses swapped digests", async () => {
    const digests = join(work, "published", "digests.txt");
    const index = await verifyImage(digests, "smoke", [
      "linux/amd64",
      "linux/arm64",
    ]);
    expect(index.repo).toBe(`${host}/zeabur-oss/smoke`);

    const swapped = join(work, "swapped.txt");
    const text = await Bun.file(digests).text();
    await writeFile(
      swapped,
      text
        .replace("linux/amd64", "linux/TMP")
        .replace("linux/arm64", "linux/amd64")
        .replace("linux/TMP", "linux/arm64"),
    );
    await expect(
      verifyImage(swapped, "smoke", ["linux/amd64", "linux/arm64"]),
    ).rejects.toThrow("the index");
  });

  test("pushChart pushes, checks and signs; verifyChart reads it back", async () => {
    const out = join(work, "published");
    const chart = join(work, "smoke-0.0.1.tgz");
    const ref = await pushChart(out, `oci://${host}/zeabur-oss/charts`, chart);
    expect(ref.repo).toBe(`${host}/zeabur-oss/charts/smoke`);
    expect(
      recorded(await readDigests(join(out, "digests.txt")), "chart"),
    ).toEqual(ref);
    expect(await calls()).toContain(`cosign sign --yes ${ref}`);

    expect(await verifyChart(join(out, "digests.txt"), chart)).toEqual(ref);
    const other = join(work, "other.tgz");
    await writeFile(other, "not the chart");
    await expect(verifyChart(join(out, "digests.txt"), other)).rejects.toThrow(
      "is not",
    );
  });

  test("checksums and verifyAssets cover exactly the assets", async () => {
    const assets = join(work, "assets");
    await mkdir(assets);
    await cp(
      join(work, "published", "digests.txt"),
      join(assets, "digests.txt"),
    );
    await cp(join(work, "smoke-0.0.1.tgz"), join(assets, "smoke-0.0.1.tgz"));
    await writeFile(join(assets, "My App.dmg"), "an app");
    await checksums(assets);
    expect(listedIn(await Bun.file(join(assets, "SHA256SUMS")).text())).toEqual(
      ["My App.dmg", "digests.txt", "smoke-0.0.1.tgz"],
    );
    await verifyAssets(assets);

    await writeFile(join(assets, "extra.txt"), "extra");
    await expect(verifyAssets(assets)).rejects.toThrow("not the assets");
  });
});

describe("the command", () => {
  const cli = join(import.meta.dir, "zeabur-oss-release.ts");

  test("notes prints the section and the docs link", async () => {
    const dir = await mkdtemp(join(tmpdir(), "release-notes-"));
    await writeFile(
      join(dir, "CHANGELOG.md"),
      "# Changelog\n\n## 0.1.1 (2026-10-28)\n\n- One fix.\n",
    );
    const out =
      await $`bun ${cli} notes ${join(dir, "CHANGELOG.md")} v0.1.1 --docs https://oss.zeabur.com/docs/builder/`.text();
    expect(out).toBe(
      "- One fix.\n\nInstall, upgrade and verify: [oss.zeabur.com/docs/builder](https://oss.zeabur.com/docs/builder/)\n",
    );
    await rm(dir, { recursive: true, force: true });
  });

  test("refuses what it does not know", async () => {
    const bad = await $`bun ${cli} push-image x`.nothrow().quiet();
    expect(bad.exitCode).toBe(1);
    expect(bad.stderr.toString()).toContain("usage:");
  });
});
