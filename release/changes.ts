// Change files and release pull requests, on knope. A pull request that users notice adds a change
// file in .changeset/; the Release pull request workflow turns the change files into the next
// CHANGELOG.md section and version, and tags the merged release pull request. The shared
// changeset.yml and release-pr.yml run these as the zeabur-oss-release command.
import { $ } from "bun";
import { Failure, section } from "./zeabur-oss-release";

function fail(message: string): never {
  throw new Failure(message);
}

const levels = ["major", "minor", "patch"];

type Package = { extra_changelog_sections?: { types?: string[] }[] };

// knope.toml's packages and the change types each takes: major, minor, patch and the types of its
// extra_changelog_sections (a `security` type for a `### Security` section). A single [package] is
// `default`, else each [packages.<name>].
export function packages(knopeToml: string): Record<string, string[]> {
  const config = Bun.TOML.parse(knopeToml) as {
    package?: Package;
    packages?: Record<string, Package>;
  };
  const named = config.package
    ? { default: config.package }
    : (config.packages ?? {});
  return Object.fromEntries(
    Object.entries(named).map(([name, pkg]) => [
      name,
      [
        ...levels,
        ...(pkg.extra_changelog_sections ?? []).flatMap((s) => s.types ?? []),
      ],
    ]),
  );
}

// What is wrong with a change file, or undefined. knope skips a file it cannot read, or a package
// it does not know, without an error, so the check is stricter than knope.
export function changeFileProblem(
  text: string,
  known: Record<string, string[]>,
): string | undefined {
  const match = text.match(/^---\n([\s\S]*?)\n---\n([\s\S]*)$/);
  if (!match) return "it starts with front matter between --- lines";
  let front: unknown;
  try {
    front = Bun.YAML.parse(match[1]);
  } catch {
    return "its front matter is not YAML";
  }
  if (typeof front !== "object" || front === null || Array.isArray(front))
    return "its front matter is `<package>: <change type>` lines";
  const entries = Object.entries(front);
  if (entries.length === 0) return "its front matter names no package";
  for (const [name, level] of entries) {
    const types = known[name];
    if (!types)
      return `${name} is not a package in knope.toml (${Object.keys(known).join(", ")})`;
    if (typeof level !== "string" || !types.includes(level))
      return `${name}: ${level} is not one of ${types.join(", ")}`;
  }
  const summary = match[2].split("\n").find((l) => l.trim() !== "");
  if (!summary?.startsWith("# ") || summary.trim() === "#")
    return "its first line after the front matter is `# <summary>`";
}

// feat, fix and perf change what users see, and so does any breaking change (`!`).
export function needsChangeFile(title: string): boolean {
  return (
    /^(feat|fix|perf)[(!:]/.test(title) || /^[a-z]+(\([^)]*\))?!:/.test(title)
  );
}

// The changeset check of a pull request: every change file it adds or changes reads, and a title
// that needs one has one, unless the pull request is labelled `changelog: none`.
export async function changeset(
  base: string,
  head: string,
  title: string,
  none: boolean,
) {
  const known = packages(await Bun.file("knope.toml").text());
  const files = (
    await $`git diff --name-only --diff-filter=AM ${`${base}...${head}`} -- ${".changeset/*.md"}`.text()
  )
    .split("\n")
    .filter(Boolean);
  const problems: string[] = [];
  for (const file of files) {
    const problem = changeFileProblem(await Bun.file(file).text(), known);
    if (problem) problems.push(`${file}: ${problem}`);
  }
  if (needsChangeFile(title) && files.length === 0 && !none)
    problems.push(
      `"${title}" needs a change file in .changeset/ (knope document-change), or the label "changelog: none" when users see no difference`,
    );
  if (problems.length > 0) fail(problems.join("\n"));
}

// Runs knope prepare-release and returns the version it wrote, vX.Y.Z or vX.Y.Z-<label>.N.
export async function prepareRelease(
  version?: string,
  prerelease?: string,
): Promise<string> {
  const want = version?.replace(/^v/, "");
  if (want && !/^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$/.test(want))
    fail(`${version} is not X.Y.Z or X.Y.Z-<pre-release>`);
  if (prerelease && !/^[a-z]+$/.test(prerelease))
    fail(
      `the pre-release label ${prerelease} is not lowercase letters, such as rc or beta`,
    );
  const args = [
    ...(want ? ["--override-version", want] : []),
    ...(prerelease ? ["--prerelease-label", prerelease] : []),
  ];
  // stdout carries only the tag; knope's log and the steps it runs go to stderr.
  await $`knope prepare-release ${args} 1>&2`;
  const changelog = await Bun.file("CHANGELOG.md").text();
  const top = changelog.match(/^## (\S+) \(/m)?.[1];
  if (!top) fail("knope wrote no ## X.Y.Z (date) section to CHANGELOG.md");
  if (want && top !== want) fail(`knope wrote ## ${top}, not ${want}`);
  section(changelog, top);
  const tag = `v${top}`;
  const tagged = await $`git rev-parse -q --verify ${`refs/tags/${tag}`}`
    .quiet()
    .nothrow();
  if (tagged.exitCode === 0) fail(`${tag} is already tagged`);
  return tag;
}

// The tag a merged release pull request's commit gets: its title, chore(release): vX.Y.Z, with the
// (#N) of a squash merge. undefined for any other commit.
export function releaseOf(subject: string): string | undefined {
  return subject.match(
    /^chore\(release\): (v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)(?: \(#\d+\))?$/,
  )?.[1];
}
