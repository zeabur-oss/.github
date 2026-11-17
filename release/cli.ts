#!/usr/bin/env bun
// The zeabur-oss-release command the shared workflows run; index.ts has the functions.
import { $ } from "bun";
import { parseArgs } from "node:util";
import { changeset, prepareRelease, releaseOf } from "./changes";
import { checksums, Failure, notes, section, verifyAssets } from "./index";

const usage = `usage:
  zeabur-oss-release notes <CHANGELOG.md> <vX.Y.Z> [--docs <url>]
  zeabur-oss-release checksums <dir>
  zeabur-oss-release verify-assets <dir>
  zeabur-oss-release changeset <base> <head> <title> [--none]
  zeabur-oss-release prepare-release [--version <vX.Y.Z>] [--prerelease <label>]
  zeabur-oss-release release-of <commit subject>`;

async function main(args: string[]) {
  let parsed;
  try {
    parsed = parseArgs({
      args,
      options: {
        docs: { type: "string" },
        none: { type: "boolean" },
        version: { type: "string" },
        prerelease: { type: "string" },
      },
      allowPositionals: true,
    });
  } catch {
    throw new Failure(usage);
  }
  const {
    values,
    positionals: [command, ...rest],
  } = parsed;
  const only = (...names: string[]) =>
    Object.keys(values).every((k) => names.includes(k));
  if (command === "notes" && rest.length === 2 && only("docs"))
    process.stdout.write(await notes(rest[0], rest[1], values.docs));
  else if (command === "checksums" && rest.length === 1 && only())
    await checksums(rest[0]);
  else if (command === "verify-assets" && rest.length === 1 && only())
    await verifyAssets(rest[0]);
  else if (command === "changeset" && rest.length === 3 && only("none"))
    await changeset(rest[0], rest[1], rest[2], values.none === true);
  else if (
    command === "prepare-release" &&
    rest.length === 0 &&
    only("version", "prerelease")
  )
    console.log(await prepareRelease(values.version, values.prerelease));
  else if (command === "release-of" && rest.length === 1 && only()) {
    const tag = releaseOf(rest[0]);
    if (tag) {
      section(await Bun.file("CHANGELOG.md").text(), tag);
      console.log(tag);
    }
  } else throw new Failure(usage);
}

try {
  await main(process.argv.slice(2));
} catch (e) {
  if (e instanceof Failure) console.error(`zeabur-oss-release: ${e.message}`);
  else if (e instanceof $.ShellError)
    console.error(`zeabur-oss-release: ${e.message}\n${e.stderr}`);
  else throw e;
  process.exit(1);
}
