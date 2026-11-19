#!/usr/bin/env bun
// The zeabur-oss-release command the shared workflows run; index.ts has the functions.
import { $ } from "bun";
import { mkdir } from "node:fs/promises";
import { parseArgs } from "node:util";
import { changeset, prepareRelease } from "./changes";
import {
  absent,
  chartPins,
  checksums,
  draft,
  Failure,
  notes,
  pinDigest,
  pushChart,
  pushImage,
  verifyAssets,
  verifyChart,
  verifyImage,
} from "./index";

const usage = `usage:
  zeabur-oss-release notes <CHANGELOG.md> <vX.Y.Z> [--docs <url>]
  zeabur-oss-release checksums <dir>
  zeabur-oss-release draft <CHANGELOG.md> <vX.Y.Z> <dir> --name <product> [--docs <url>]
  zeabur-oss-release verify-assets <dir>
  zeabur-oss-release absent <ref>...
  zeabur-oss-release push-image --out <dir> [--label <name>] [--pin <values.yaml>:<key>] <repo:tag> <os/arch=archive.tar>...
  zeabur-oss-release push-chart --out <dir> <oci://repo> <chart.tgz>
  zeabur-oss-release pin-digest <values.yaml> <key.path> <sha256:digest>
  zeabur-oss-release verify-image --digests <file> [--label <name>] [--chart <chart.tgz> --key <key>] <os/arch>...
  zeabur-oss-release verify-chart --digests <file> <chart.tgz>
  zeabur-oss-release changeset <base> <head> <title> [--none]
  zeabur-oss-release prepare-release [--version <vX.Y.Z>] [--prerelease <label>]`;

async function main(args: string[]) {
  let parsed;
  try {
    parsed = parseArgs({
      args,
      options: {
        docs: { type: "string" },
        name: { type: "string" },
        none: { type: "boolean" },
        version: { type: "string" },
        prerelease: { type: "string" },
        out: { type: "string" },
        label: { type: "string" },
        digests: { type: "string" },
        pin: { type: "string" },
        chart: { type: "string" },
        key: { type: "string" },
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
  else if (
    command === "draft" &&
    rest.length === 3 &&
    values.name &&
    only("name", "docs")
  )
    await draft(rest[0], rest[1], rest[2], values.name, values.docs);
  else if (command === "verify-assets" && rest.length === 1 && only())
    await verifyAssets(rest[0]);
  else if (command === "absent" && rest.length > 0 && only())
    await absent(...rest);
  else if (
    command === "push-image" &&
    rest.length >= 2 &&
    values.out &&
    only("out", "label", "pin")
  ) {
    const archives = Object.fromEntries(
      rest.slice(1).map((a) => {
        const at = a.indexOf("=");
        if (at < 1) throw new Failure(usage);
        return [a.slice(0, at), a.slice(at + 1)];
      }),
    );
    await mkdir(values.out, { recursive: true });
    const index = await pushImage(
      values.out,
      values.label ?? "",
      rest[0],
      archives,
    );
    if (values.pin) {
      const [file, key] = values.pin.split(":");
      const valuesFile = Bun.file(file);
      await Bun.write(
        valuesFile,
        pinDigest(await valuesFile.text(), key.split("."), index.digest),
      );
    }
    console.log(String(index));
  } else if (
    command === "push-chart" &&
    rest.length === 2 &&
    values.out &&
    only("out")
  ) {
    await mkdir(values.out, { recursive: true });
    console.log(String(await pushChart(values.out, rest[0], rest[1])));
  } else if (command === "pin-digest" && rest.length === 3 && only()) {
    const file = Bun.file(rest[0]);
    await Bun.write(
      file,
      pinDigest(await file.text(), rest[1].split("."), rest[2]),
    );
  } else if (
    command === "verify-image" &&
    rest.length > 0 &&
    values.digests &&
    only("digests", "label", "chart", "key") &&
    !values.chart === !values.key
  ) {
    const index = await verifyImage(values.digests, values.label ?? "", rest);
    if (values.chart && values.key)
      await chartPins(values.chart, values.key, index);
    console.log(String(index));
  } else if (
    command === "verify-chart" &&
    rest.length === 1 &&
    values.digests &&
    only("digests")
  )
    console.log(String(await verifyChart(values.digests, rest[0])));
  else if (command === "changeset" && rest.length === 3 && only("none"))
    await changeset(rest[0], rest[1], rest[2], values.none === true);
  else if (
    command === "prepare-release" &&
    rest.length === 0 &&
    only("version", "prerelease")
  )
    console.log(await prepareRelease(values.version, values.prerelease));
  else throw new Failure(usage);
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
