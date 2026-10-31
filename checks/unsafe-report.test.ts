// The unsafe report on a workspace of one binary that depends on `risky`, a crate outside the
// workspace with one `unsafe` line and no native code. Run with `bun test checks/` in the dev shell.
import { $ } from "bun";
import { afterAll, beforeAll, describe, expect, test } from "bun:test";
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

const script = join(import.meta.dir, "unsafe-report.sh");

const entry = (name: string, kind: string, approved = "2026-11-01") =>
  `[[crate]]\nname = "${name}"\nkind = "${kind}"\nreason = "Test"\napproved = "${approved}"\n`;

describe("unsafe report", () => {
  let work: string;
  let app: string;

  beforeAll(async () => {
    work = await mkdtemp(join(tmpdir(), "unsafe-report-"));
    const risky = join(work, "risky");
    app = join(work, "app");
    await mkdir(join(risky, "src"), { recursive: true });
    await writeFile(
      join(risky, "Cargo.toml"),
      '[package]\nname = "risky"\nversion = "0.1.0"\nedition = "2021"\n',
    );
    await writeFile(join(risky, "src", "lib.rs"), "pub unsafe fn f() {}\n");
    await mkdir(join(app, "src"), { recursive: true });
    await mkdir(join(app, "supply-chain"));
    await writeFile(
      join(app, "Cargo.toml"),
      '[package]\nname = "app"\nversion = "0.1.0"\nedition = "2021"\n\n[dependencies]\nrisky = { path = "../risky" }\n',
    );
    await writeFile(join(app, "src", "main.rs"), "fn main() {}\n");
    await $`git init -q && cargo generate-lockfile --offline --quiet`.cwd(app);
    // Cargo's first run on a cold runner takes longer than a test.
  }, 60_000);

  afterAll(async () => {
    await rm(work, { recursive: true, force: true });
  });

  const run = async (allowlist: string) => {
    await writeFile(
      join(app, "supply-chain", "unsafe-allowlist.toml"),
      allowlist,
    );
    const r = await $`bash ${script} app`.cwd(app).nothrow().quiet();
    if (r.exitCode > 1) throw new Error(r.stderr.toString());
    return {
      code: r.exitCode,
      report: await Bun.file(join(app, "target", "unsafe-report.md")).text(),
      stderr: r.stderr.toString(),
    };
  };

  test("passes when the allowlist has exactly what the graph needs", async () => {
    const r = await run(entry("risky", "unsafe"));
    expect(r.code).toBe(0);
    expect(r.report).toContain("| risky | 0.1.0 | 1 | no | yes |");
    expect(r.report).not.toContain("doesn't need");
  });

  test("fails on a crate without an approval", async () => {
    const r = await run(entry("risky", "unsafe", "pending"));
    expect(r.code).toBe(1);
    expect(r.report).toContain("| risky | 0.1.0 | 1 | no | NO |");
  });

  test("fails on entries the graph doesn't need", async () => {
    const r = await run(
      entry("risky", "unsafe") +
        entry("risky", "native") +
        entry("gone", "unsafe"),
    );
    expect(r.code).toBe(1);
    expect(r.report).toContain("| risky | 0.1.0 | 1 | no | yes |");
    expect(r.report).toContain(
      "## Entries the shipped graph doesn't need\n\n| crate | kind |\n|---|---|\n| gone | unsafe |\n| risky | native |\n",
    );
    expect(r.stderr).toContain("remove the entries");
  });
});
