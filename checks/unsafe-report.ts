#!/usr/bin/env bun
// Lists every shipped dependency that contains `unsafe` code or links native code, and fails if one
// is not approved in the repository's supply-chain/unsafe-allowlist.toml, or if the allowlist has an
// entry the shipped graph doesn't need. Writes a Markdown report to target/unsafe-report.md. Policy:
// CONTRIBUTING.md, "Dependencies".
//
// The `unsafe` count is the number of non-comment source lines containing the `unsafe` keyword in
// the crate's .rs files. It is a review signal, not an audit. The dependency graph comes from the
// caller's pinned Cargo, so the command keeps working when the manifest uses new Cargo features.
//
// Run from the Cargo workspace, with the packages whose binaries ship:
//   zeabur-oss-unsafe-report zeabur-identity zeabur-identityctl
import { $ } from "bun";
import { mkdir } from "node:fs/promises";
import { join } from "node:path";

const target = "x86_64-unknown-linux-musl";

type Kind = "unsafe" | "native";
type Entry = { name: string; kind: Kind; approved?: string };

const keyword = /(^|[^A-Za-z0-9_])unsafe([^A-Za-z0-9_]|$)/;
const comment = /^\s*(\/\/|\*)/;

// Non-comment lines with the `unsafe` keyword in the .rs files under dir.
export async function unsafeLines(dir: string): Promise<number> {
  let n = 0;
  for await (const file of new Bun.Glob("**/*.rs").scan({
    cwd: dir,
    dot: true,
  })) {
    for (const line of (await Bun.file(join(dir, file)).text()).split("\n")) {
      if (keyword.test(line) && !comment.test(line)) n++;
    }
  }
  return n;
}

async function main(shippedPackages: string[]): Promise<number> {
  if (shippedPackages.length === 0) {
    console.error("usage: zeabur-oss-unsafe-report <shipped package>...");
    return 2;
  }
  const root = (await $`git rev-parse --show-toplevel`.text()).trim();
  const allowlist = join(root, "supply-chain", "unsafe-allowlist.toml");
  const reportFile = join(root, "target", "unsafe-report.md");
  await mkdir(join(root, "target"), { recursive: true });

  const entries: Entry[] =
    (Bun.TOML.parse(await Bun.file(allowlist).text()) as { crate?: Entry[] })
      .crate ?? [];
  // An entry counts only when `approved` is a date; "pending" marks a proposal awaiting a
  // maintainer's review.
  const approved = (name: string, kind: Kind) =>
    entries.some(
      (e) =>
        e.name === name &&
        e.kind === kind &&
        /^\d{4}-\d{2}-\d{2}$/.test(e.approved ?? ""),
    );

  // Download sources first; the scan reads them from the Cargo home.
  await $`cargo fetch --locked --quiet`;

  // The shipped graph: normal and build dependencies of the packages given, for the image target.
  // Dev-dependencies and the test-only crates never reach a binary and are not listed.
  const tree =
    await $`cargo tree --locked ${shippedPackages.map((p) => `--package=${p}`)} --all-features -e normal,build --target ${target} --prefix none --format ${"{p}"}`.text();
  const shipped = new Set(
    tree
      .split("\n")
      .filter(Boolean)
      .map((l) => l.replace(/ \(.*/, "").replace(" v", "@")),
  );

  const metadata =
    await $`cargo metadata --format-version 1 --locked --all-features --filter-platform ${target}`.json();
  const workspace = new Set<string>(metadata.workspace_members);

  const needed = new Set<string>();
  let missing = false;
  const rows: string[] = [];
  for (const p of metadata.packages) {
    if (!shipped.has(`${p.name}@${p.version}`) || workspace.has(p.id)) continue;
    const dir = p.manifest_path.replace(/\/Cargo\.toml$/, "");
    if (!(await Bun.file(p.manifest_path).exists())) {
      console.error(
        `error: sources of ${p.name} ${p.version} not found at ${dir}`,
      );
      return 1;
    }
    const used = await unsafeLines(dir);
    const native = Boolean(p.links);
    if (used === 0 && !native) continue;
    let ok = true;
    for (const [kind, has] of [
      ["unsafe", used > 0],
      ["native", native],
    ] as const) {
      if (!has) continue;
      needed.add(`${p.name} ${kind}`);
      if (!approved(p.name, kind)) ok = false;
    }
    if (!ok) missing = true;
    rows.push(
      `| ${p.name} | ${p.version} | ${used} | ${native ? "yes" : "no"} | ${ok ? "yes" : "NO"} |`,
    );
  }

  const unneeded = [...new Set(entries.map((e) => `${e.name} ${e.kind}`))]
    .filter((e) => !needed.has(e))
    .sort();

  const report = [
    "# Dependency unsafe report",
    "",
    "| crate | version | lines with `unsafe` | native code | approved |",
    "|---|---|---|---|---|",
    ...rows,
  ];
  if (unneeded.length > 0) {
    report.push(
      "",
      "## Entries the shipped graph doesn't need",
      "",
      "| crate | kind |",
      "|---|---|",
      ...unneeded.map((e) => `| ${e.replace(" ", " | ")} |`),
    );
  }
  const text = report.join("\n") + "\n";
  await Bun.write(reportFile, text);
  process.stdout.write(text);

  if (missing) {
    console.error(
      `error: dependencies above marked NO need a maintainer's approval in ${allowlist}`,
    );
  }
  if (unneeded.length > 0) {
    console.error(
      `error: remove the entries the shipped graph doesn't need from ${allowlist}`,
    );
  }
  return missing || unneeded.length > 0 ? 1 : 0;
}

if (import.meta.main) {
  try {
    process.exit(await main(process.argv.slice(2)));
  } catch (e) {
    if (e instanceof $.ShellError) process.exit(e.exitCode);
    throw e;
  }
}
