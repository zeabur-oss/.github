import { describe, expect, test } from "bun:test";
import { matches, matrix, members, parse } from "./matrix.ts";

const jobs = (
  table: string,
  tier: "fast" | "all" = "all",
  changed: string[] = [],
) =>
  matrix(parse(table), tier, changed).map(
    (row) => (row as { job: string }).job,
  );

describe("defaults", () => {
  const table = parse(`
    [defaults]
    rust-cache = true
    runner = "ubuntu-26.04"

    [jobs.test]
    [jobs.ui]
    fetch-depth = 2
    runner = "ubuntu-26.04-arm"
  `);

  test("a job gets the defaults and its own settings win", () => {
    expect(matrix(table, "all")).toEqual([
      { job: "test", "rust-cache": "true", runner: "ubuntu-26.04" },
      {
        job: "ui",
        "rust-cache": "true",
        runner: "ubuntu-26.04-arm",
        "fetch-depth": "2",
      },
    ]);
  });

  test("tier, in, shell and skip-if-only stay out of the matrix", () => {
    const row = matrix(
      parse(`
        [defaults]
        skip-if-only = ["docs/*"]
        [jobs.wording]
        tier = "merge"
        shell = "zh"
      `),
      "all",
    )[0];
    expect(row).toEqual({ job: "wording" });
  });
});

describe("skip-if-only", () => {
  const table = `
    [defaults]
    skip-if-only = ["docs/*", "*.md"]

    [jobs.test]
    [jobs.docs-check]
    skip-if-only = []
  `;

  test("* matches across /", () => {
    expect(matches("docs/*", "docs/a/b/c.mdx")).toBe(true);
    expect(matches("*.md", "crates/x/README.md")).toBe(true);
    expect(matches("docs/*", "src/docs/a")).toBe(false);
  });

  test("the rest of a pattern is literal", () => {
    expect(matches("CHANGELOG.md", "CHANGELOGxmd")).toBe(false);
    expect(matches(".changeset/*", ".changeset/a.md")).toBe(true);
  });

  test("a job whose changed files all match is left out", () => {
    expect(jobs(table, "fast", ["docs/a.mdx", "README.md"])).toEqual([
      "docs-check",
    ]);
  });

  test("one file that matches nothing keeps the job", () => {
    expect(jobs(table, "fast", ["docs/a.mdx", "src/main.rs"])).toEqual([
      "test",
      "docs-check",
    ]);
  });

  test("an empty value disables the default", () => {
    expect(jobs(table, "fast", ["README.md"])).toEqual(["docs-check"]);
  });

  test("without changed files nothing is left out", () => {
    expect(jobs(table, "all")).toEqual(["test", "docs-check"]);
  });
});

describe("tiers", () => {
  const table = `
    [jobs.test]
    [jobs.kubernetes]
    tier = "merge"
  `;

  test("fast leaves out merge jobs", () => {
    expect(jobs(table, "fast")).toEqual(["test"]);
  });

  test("all takes every job", () => {
    expect(jobs(table, "all")).toEqual(["test", "kubernetes"]);
  });
});

describe("groups", () => {
  const table = parse(`
    [jobs.lint]
    [jobs.format]
    in = "lint"
    [jobs.clippy]
    in = "lint"
    [jobs.test]
  `);

  test("members run inside the group, not as jobs of their own", () => {
    expect(
      matrix(table, "all").map((row) => (row as { job: string }).job),
    ).toEqual(["lint", "test"]);
  });

  test("members lists them in table order", () => {
    expect(members(table, "lint")).toEqual(["format", "clippy"]);
    expect(members(table, "test")).toEqual([]);
  });
});

describe("empty matrix", () => {
  test("no jobs", () => {
    expect(JSON.stringify(matrix(parse(""), "all"))).toBe("[]");
  });

  test("every job skipped", () => {
    const table = `
      [defaults]
      skip-if-only = ["docs/*"]
      [jobs.test]
    `;
    expect(JSON.stringify(matrix(parse(table), "fast", ["docs/a.md"]))).toBe(
      "[]",
    );
  });
});
