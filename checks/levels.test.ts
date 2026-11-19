import { describe, expect, test } from "bun:test";
import { levelErrors, type Metadata } from "./levels";

const pkg = (name: string, ...deps: string[]) => ({
  name,
  dependencies: deps.map((d) => {
    const [dep, kind] = d.split(":");
    return { name: dep, kind: kind ?? null };
  }),
});

const workspace = (
  metadata: Metadata["metadata"],
  ...packages: ReturnType<typeof pkg>[]
): Metadata => ({ packages, metadata });

describe("levelErrors", () => {
  test("accepts dependencies on lower levels, and any dev-dependency", () => {
    expect(
      levelErrors(
        workspace(
          { levels: { core: 0, app: 1, helpers: 2 } },
          pkg("core", "serde", "helpers:dev"),
          pkg("app", "core", "core:build", "helpers:dev"),
          pkg("helpers", "app"),
        ),
      ),
    ).toEqual([]);
  });

  test("refuses a crate without a level, and a dependency on the same or a higher level", () => {
    expect(
      levelErrors(
        workspace(
          { levels: { core: 0, other: 0 } },
          pkg("core", "other"),
          pkg("other"),
          pkg("new"),
        ),
      ),
    ).toEqual([
      "core (level 0) depends on other (level 0)",
      "new has no level",
    ]);
    expect(levelErrors(workspace(null, pkg("core")))).toEqual([
      "Cargo.toml has no [workspace.metadata.levels]",
    ]);
  });

  test("lets only a composer depend on a module crate", () => {
    const modules = { crates: ["module"], composers: ["server"] };
    expect(
      levelErrors(
        workspace(
          { levels: { module: 0, api: 1, server: 2 }, modules },
          pkg("module"),
          pkg("api", "module"),
          pkg("server", "module", "api"),
        ),
      ),
    ).toEqual(["api depends on the module crate module; only server may"]);
  });

  test("keeps a pure crate to pure crates and the listed third-party crates", () => {
    const levels = { model: 0, kube: 0, plan: 1 };
    const packages = [
      pkg("model", "serde"),
      pkg("kube", "tokio"),
      pkg("plan", "model", "kube", "tokio", "tempfile:dev"),
    ];
    expect(
      levelErrors(
        workspace({ levels, pure: { crates: ["model", "plan"] } }, ...packages),
      ),
    ).toEqual(["the pure crate plan depends on kube, which is not pure"]);
    expect(
      levelErrors(
        workspace(
          {
            levels,
            pure: { crates: ["model", "plan"], "third-party": ["serde"] },
          },
          ...packages,
        ),
      ),
    ).toEqual([
      "the pure crate plan depends on kube, which is not pure",
      "the pure crate plan depends on tokio, which is not in [workspace.metadata.pure] third-party",
    ]);
  });

  test("keeps an isolated crate to the workspace crates listed for it", () => {
    expect(
      levelErrors(
        workspace(
          {
            levels: { http: 0, model: 0, proxy: 1, cli: 1 },
            isolated: { proxy: [], cli: ["http"] },
          },
          pkg("http"),
          pkg("model"),
          pkg("proxy", "http", "hyper"),
          pkg("cli", "http", "model"),
        ),
      ),
    ).toEqual([
      "the isolated crate cli depends on model; it may depend only on http",
      "the isolated crate proxy depends on http; it may depend only on no workspace crate",
    ]);
  });
});
