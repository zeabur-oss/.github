// The change file rules, then prepareRelease with knope in a scratch repository. Run with
// `bun test release/changes.test.ts` in the dev shell; it needs no Docker.
import { $ } from "bun";
import {
  afterAll,
  afterEach,
  beforeAll,
  describe,
  expect,
  test,
} from "bun:test";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  changeFileProblem,
  needsChangeFile,
  packages,
  prepareRelease,
  released,
} from "./changes";
import { Failure } from "./index";

describe("packages", () => {
  test("a single [package] is default", () => {
    expect(packages('[package]\nchangelog = "CHANGELOG.md"\n')).toEqual({
      default: ["major", "minor", "patch"],
    });
  });
  test("extra_changelog_sections add their types", () => {
    expect(
      packages(
        '[package]\nextra_changelog_sections = [{ name = "Security", types = ["security"] }]\n',
      ).default,
    ).toContain("security");
  });
  test("[packages.<name>] are named", () => {
    expect(
      packages(
        '[packages.ui]\nchangelog = "a"\n[packages."zeabur-cli"]\nchangelog = "b"\n',
      ),
    ).toContainAllKeys(["ui", "zeabur-cli"]);
  });
});

describe("changeFileProblem", () => {
  const known = { default: ["major", "minor", "patch", "security"] };
  test("a well-formed file passes", () => {
    expect(
      changeFileProblem(
        "---\ndefault: minor\n---\n\n# Skeletons show at once\n\nMore.\n",
        known,
        true,
      ),
    ).toBeUndefined();
  });
  test("a type of an extra changelog section passes", () => {
    expect(
      changeFileProblem(
        "---\ndefault: security\n---\n\n# A hole\n",
        known,
        true,
      ),
    ).toBeUndefined();
  });
  test.each([
    ["no front matter", "# Summary\n"],
    ["an unknown package", "---\nui: patch\n---\n\n# Summary\n"],
    ["an unknown type", "---\ndefault: breaking\n---\n\n# Summary\n"],
    ["no summary heading", "---\ndefault: patch\n---\n\nSummary\n"],
    ["an empty heading", "---\ndefault: patch\n---\n\n#\n"],
    ["empty front matter", "---\n\n---\n\n# Summary\n"],
  ])("%s fails", (_, text) => {
    expect(changeFileProblem(text, known, true)).toBeString();
  });
  test("major waits for the first release", () => {
    const major = "---\ndefault: major\n---\n\n# A rename\n";
    expect(changeFileProblem(major, known, true)).toBeUndefined();
    expect(changeFileProblem(major, known, false)).toBeString();
  });
});

describe("released", () => {
  test("a release section counts, the introduction does not", () => {
    expect(released("# Changelog\n\nIntro.\n")).toBe(false);
    expect(released("# Changelog\n\nIntro.\n\n## 0.1.0 (2026-11-20)\n")).toBe(
      true,
    );
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

const knopeToml = `[package]
versioned_files = [{ path = "Cargo.toml", regex = '\\[workspace\\.package\\]\\nversion = "(?<version>[^"]+)"' }]
changelog = "CHANGELOG.md"

[changes]
ignore_conventional_commits = true

[[workflows]]
name = "prepare-release"

[[workflows.steps]]
type = "PrepareRelease"

# A caller's own step may print.
[[workflows.steps]]
type = "Command"
command = "echo set $version"
variables = { "$version" = "Version" }
`;

const git = (args: string[]) =>
  $`git -c user.name=t -c user.email=t@example.com -c commit.gpgsign=false -c tag.gpgsign=false ${args}`.quiet();

// A scratch repository at version 0.1.1 with one change file, as the working directory.
async function scratch(changelog: string, change: string): Promise<string> {
  const dir = await mkdtemp(join(tmpdir(), "changes-"));
  process.chdir(dir);
  await Bun.write("Cargo.toml", '[workspace.package]\nversion = "0.1.1"\n');
  await Bun.write("CHANGELOG.md", changelog);
  await Bun.write("knope.toml", knopeToml);
  await Bun.write(".changeset/a.md", change);
  await $`git init -q`;
  await git(["add", "-A"]);
  await git(["commit", "-qm", "init"]);
  return dir;
}

describe("prepareRelease", () => {
  let dir: string;
  const cwd = process.cwd();

  beforeAll(async () => {
    dir = await scratch(
      "# Changelog\n\n## 0.1.1 (2026-10-20)\n\n- Old.\n",
      "---\ndefault: minor\n---\n\n# A thing\n",
    );
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
  test("knope's output stays out of the printed tag", async () => {
    const cli = join(cwd, "release/cli.ts");
    const out = await $`bun ${cli} prepare-release`.text();
    expect(out).toBe("v0.1.2\n");
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

describe("prepareRelease of the first release", () => {
  const cwd = process.cwd();
  const intro = "# Changelog\n\nAll notable changes.\n";
  let dir: string;
  afterEach(async () => {
    process.chdir(cwd);
    await rm(dir, { recursive: true, force: true });
  });

  test("the section starts after a blank line", async () => {
    dir = await scratch(intro, "---\ndefault: minor\n---\n\n# A thing\n");
    await prepareRelease("v0.1.0");
    expect(await Bun.file("CHANGELOG.md").text()).toStartWith(
      `${intro}\n## 0.1.0 (`,
    );
  });
  test("a major change is refused", async () => {
    dir = await scratch(intro, "---\ndefault: major\n---\n\n# A rename\n");
    await expect(prepareRelease("v0.1.0")).rejects.toThrow(Failure);
  });
});
