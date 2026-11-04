#!/usr/bin/env bun
// The job table of scripts/ci/jobs.toml, as zeabur-oss/.github's shared CI workflow and
// scripts/ci.sh read it. This file is the same in every zeabur-oss repository.
//
//   bun scripts/ci/matrix.ts fast|all [changed-files]   # the workflow's matrix, as JSON
//   bun scripts/ci/matrix.ts list                       # every job name
//   bun scripts/ci/matrix.ts members <group>            # the jobs that run inside <group>
//   bun scripts/ci/matrix.ts get <job> <key>            # one setting of a job; exit 1 if no such job
import { join } from "node:path";

type Value = string | number | boolean | string[];
type Settings = Record<string, Value>;
export type Table = { defaults?: Settings; jobs?: Record<string, Settings> };
type Entry = { job: string; settings: Settings };

// Settings the matrix does not pass on to the workflow.
const own = new Set(["tier", "in", "shell", "skip-if-only"]);

export function parse(text: string): Table {
  return Bun.TOML.parse(text) as Table;
}

// Each job with the defaults under its own settings, in the table's order.
export function entries(table: Table): Entry[] {
  return Object.entries(table.jobs ?? {}).map(([job, settings]) => ({
    job,
    settings: { ...table.defaults, ...settings },
  }));
}

// `*` also matches `/`; the rest of the pattern is literal.
export function matches(pattern: string, path: string): boolean {
  const source = pattern
    .split("*")
    .map((part) => part.replace(/[.+?^${}()|[\]\\]/g, "\\$&"))
    .join(".*");
  return new RegExp(`^${source}$`).test(path);
}

export function matrix(
  table: Table,
  tier: "fast" | "all",
  changed: string[] = [],
): object[] {
  const out: object[] = [];
  for (const { job, settings } of entries(table)) {
    if ("in" in settings) continue;
    if (tier === "fast" && (settings.tier ?? "fast") !== "fast") continue;
    const skip = [settings["skip-if-only"] ?? []].flat().map(String);
    if (
      skip.length > 0 &&
      changed.length > 0 &&
      changed.every((path) => skip.some((pattern) => matches(pattern, path)))
    ) {
      continue;
    }
    const passed = Object.entries(settings)
      .filter(([key]) => !own.has(key))
      .map(([key, value]) => [key, String(value)]);
    out.push({ job, ...Object.fromEntries(passed) });
  }
  return out;
}

export function members(table: Table, group: string): string[] {
  return entries(table)
    .filter(({ settings }) => settings.in === group)
    .map(({ job }) => job);
}

async function main(args: string[]) {
  const path = process.env.CI_JOBS ?? join(import.meta.dir, "jobs.toml");
  const table = parse(await Bun.file(path).text());
  const [command, a, b] = args;
  switch (command) {
    case "fast":
    case "all": {
      const changed = a
        ? (await Bun.file(a).text()).split("\n").filter(Boolean)
        : [];
      console.log(JSON.stringify(matrix(table, command, changed)));
      return;
    }
    case "list":
      for (const { job } of entries(table)) console.log(job);
      return;
    case "members":
      for (const job of members(table, a ?? "")) console.log(job);
      return;
    case "get": {
      const entry = entries(table).find(({ job }) => job === a);
      if (!entry) process.exit(1);
      const value = entry.settings[b ?? ""];
      if (value !== undefined) console.log([value].flat().join(","));
      return;
    }
    default:
      console.error(
        "usage: matrix.ts fast|all [changed-files] | list | members <group> | get <job> <key>",
      );
      process.exit(2);
  }
}

if (import.meta.main) await main(process.argv.slice(2));
