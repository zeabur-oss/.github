#!/usr/bin/env bun
// Checks the crate structure a Cargo workspace declares in Cargo.toml [workspace.metadata]:
//
// - levels: every workspace crate has one, and depends only on workspace crates of a lower level.
// - modules (crates, composers): only a composer depends on a module crate.
// - pure (crates, third-party): a pure crate depends only on pure workspace crates and, when
//   third-party is given, only on those third-party crates.
// - isolated: each listed crate depends only on the workspace crates listed for it.
//
// Normal and build dependencies count; dev-dependencies do not, so tests may use test helpers.
// Run from the Cargo workspace: zeabur-oss-levels
import { $ } from "bun";

type Dependency = { name: string; kind: string | null };
type Package = { name: string; dependencies: Dependency[] };
export type Metadata = {
  packages: Package[];
  metadata: {
    levels?: Record<string, number>;
    modules?: { crates: string[]; composers: string[] };
    pure?: { crates: string[]; "third-party"?: string[] };
    isolated?: Record<string, string[]>;
  } | null;
};

// Every way the workspace breaks its declared structure, one line each.
export function levelErrors({ packages, metadata }: Metadata): string[] {
  const levels = metadata?.levels;
  if (!levels) return ["Cargo.toml has no [workspace.metadata.levels]"];
  const { modules, pure, isolated = {} } = metadata;
  const members = new Set(packages.map((p) => p.name));
  const errors: string[] = [];
  for (const p of packages) {
    if (levels[p.name] === undefined) errors.push(`${p.name} has no level`);
    const deps = [
      ...new Set(
        p.dependencies.filter((d) => d.kind !== "dev").map((d) => d.name),
      ),
    ];
    for (const dep of deps) {
      const inWorkspace = members.has(dep);
      if (inWorkspace && levels[dep] >= levels[p.name])
        errors.push(
          `${p.name} (level ${levels[p.name]}) depends on ${dep} (level ${levels[dep]})`,
        );
      if (
        modules?.crates.includes(dep) &&
        !modules.crates.includes(p.name) &&
        !modules.composers.includes(p.name)
      )
        errors.push(
          `${p.name} depends on the module crate ${dep}; only ${modules.composers.join(", ")} may`,
        );
      if (pure?.crates.includes(p.name)) {
        if (inWorkspace && !pure.crates.includes(dep))
          errors.push(
            `the pure crate ${p.name} depends on ${dep}, which is not pure`,
          );
        const thirdParty = pure["third-party"];
        if (!inWorkspace && thirdParty && !thirdParty.includes(dep))
          errors.push(
            `the pure crate ${p.name} depends on ${dep}, which is not in [workspace.metadata.pure] third-party`,
          );
      }
      const allowed = isolated[p.name];
      if (allowed && inWorkspace && !allowed.includes(dep))
        errors.push(
          `the isolated crate ${p.name} depends on ${dep}; it may depend only on ${allowed.join(", ") || "no workspace crate"}`,
        );
    }
  }
  return errors.sort();
}

if (import.meta.main) {
  const metadata: Metadata =
    await $`cargo metadata --format-version 1 --no-deps --locked`.json();
  const errors = levelErrors(metadata);
  for (const error of errors) console.error(`error: ${error}`);
  if (errors.length > 0) process.exit(1);
  console.log("crate levels: ok");
}
