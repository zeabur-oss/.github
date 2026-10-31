// The change file rules, then prepareRelease with knope in a scratch repository. Run with
// `bun test release/changes.test.ts` in the dev shell; it needs no Docker.
import { $ } from "bun";
import { afterAll, beforeAll, describe, expect, test } from "bun:test";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  changeFileProblem,
  needsChangeFile,
  packages,
  prepareRelease,
  releaseOf,
} from "./changes";
import { Failure } from "./zeabur-oss-release";

describe("packages", () => {
  test("a single [package] is default", () => {
    expect(packages('[package]\nchangelog = "CHANGELOG.md"\n')).toEqual([
      "default",
    ]);
  });
  test("[packages.<name>] are named", () => {
    expect(
      packages(
        '[packages.ui]\nchangelog = "a"\n[packages."zeabur-cli"]\nchangelog = "b"\n',
      ),
    ).toEqual(["ui", "zeabur-cli"]);
  });
});

describe("changeFileProblem", () => {
  const known = ["default"];
  test("a well-formed file passes", () => {
    expect(
      changeFileProblem(
        "---\ndefault: minor\n---\n\n# Skeletons show at once\n\nMore.\n",
        known,
      ),
    ).toBeUndefined();
  });
  test.each([
    ["no front matter", "# Summary\n"],
    ["an unknown package", "---\nui: patch\n---\n\n# Summary\n"],
    ["an unknown level", "---\ndefault: breaking\n---\n\n# Summary\n"],
    ["no summary heading", "---\ndefault: patch\n---\n\nSummary\n"],
    ["an empty heading", "---\ndefault: patch\n---\n\n#\n"],
    ["empty front matter", "---\n\n---\n\n# Summary\n"],
  ])("%s fails", (_, text) => {
    expect(changeFileProblem(text, known)).toBeString();
  });
});

describe("needsChangeFile", () => {
  test.each([
    "feat: a",
    "fix(ui): a",
    "perf!: a",
    "ci!: drop an input",
    "refactor(api)!: a",
  ])("%s needs one", (title) => expect(needsChangeFile(title)).toBe(true));
  test.each(["ci: a", "chore(deps): a", "docs: feature list", "feature: a"])(
    "%s does not",
    (title) => expect(needsChangeFile(title)).toBe(false),
  );
});

describe("releaseOf", () => {
  test("a squash-merged release pull request", () => {
    expect(releaseOf("chore(release): v0.1.2 (#13)")).toBe("v0.1.2");
    expect(releaseOf("chore(release): v0.2.0-rc.1")).toBe("v0.2.0-rc.1");
  });
  test.each([
    "chore(release): 0.1.2",
    "chore(release): v0.1.2 and more",
    "fix: v0.1.2",
  ])("%s is not one", (subject) => expect(releaseOf(subject)).toBeUndefined());
});

describe("prepareRelease", () => {
  let dir: string;
  const cwd = process.cwd();
  const git = (args: string[]) =>
    $`git -c user.name=t -c user.email=t@example.com -c commit.gpgsign=false -c tag.gpgsign=false ${args}`.quiet();

  beforeAll(async () => {
    dir = await mkdtemp(join(tmpdir(), "changes-"));
    process.chdir(dir);
    await Bun.write("Cargo.toml", '[workspace.package]\nversion = "0.1.1"\n');
    await Bun.write(
      "CHANGELOG.md",
      "# Changelog\n\n## 0.1.1 (2026-10-20)\n\n- Old.\n",
    );
    await Bun.write(
      "knope.toml",
      `[package]
versioned_files = [{ path = "Cargo.toml", regex = '\\[workspace\\.package\\]\\nversion = "(?<version>[^"]+)"' }]
changelog = "CHANGELOG.md"

[changes]
ignore_conventional_commits = true

[[workflows]]
name = "prepare-release"

[[workflows.steps]]
type = "PrepareRelease"
`,
    );
    await Bun.write(
      ".changeset/a.md",
      "---\ndefault: minor\n---\n\n# A thing\n",
    );
    await $`git init -q`;
    await git(["add", "-A"]);
    await git(["commit", "-qm", "init"]);
    await git(["tag", "-a", "-m", "v0.1.1", "v0.1.1"]);
  });
  afterAll(async () => {
    process.chdir(cwd);
    await rm(dir, { recursive: true, force: true });
  });
  const reset = () => git(["reset", "-q", "--hard"]);

  test("the change files give the version, a minor change before 1.0 a patch", async () => {
    expect(await prepareRelease()).toBe("v0.1.2");
    expect(await Bun.file("CHANGELOG.md").text()).toContain("## 0.1.2 (");
    await reset();
  });
  test("a pre-release label", async () => {
    expect(await prepareRelease(undefined, "rc")).toBe("v0.1.2-rc.0");
    await reset();
  });
  test("an override", async () => {
    expect(await prepareRelease("v0.2.0")).toBe("v0.2.0");
    await reset();
  });
  test("a version already tagged is refused", async () => {
    await git(["tag", "-a", "-m", "v0.1.2", "v0.1.2", "HEAD"]);
    await expect(prepareRelease()).rejects.toThrow(Failure);
    await reset();
    await git(["tag", "-d", "v0.1.2"]);
  });
  test("bad inputs are refused before knope runs", async () => {
    await expect(prepareRelease("0.2")).rejects.toThrow(Failure);
    await expect(prepareRelease(undefined, "RC1")).rejects.toThrow(Failure);
  });
});
