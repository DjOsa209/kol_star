import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import test from "node:test";

test("build info hook does not crash when the output directory is absent", () => {
  const script = `
    import { loadConfigFromFile } from "vite";
    const loaded = await loadConfigFromFile(
      { command: "build", mode: "production" },
      "./vite.config.ts"
    );
    const plugins = (await Promise.all(loaded.config.plugins))
      .flat(Infinity)
      .filter(Boolean);
    const plugin = plugins.find(item => item.name === "vite:buildInfo");
    plugin.configResolved({
      command: "build",
      root: process.cwd(),
      build: { outDir: "__missing_dist_regression__" }
    });
    plugin.buildStart();
    const outputHook = plugin.writeBundle ?? plugin.closeBundle;
    await outputHook.call(plugin);
    await new Promise(resolve => setTimeout(resolve, 100));
  `;
  const result = spawnSync(
    process.execPath,
    ["--input-type=module", "--eval", script],
    { cwd: process.cwd(), encoding: "utf8" }
  );

  assert.equal(result.status, 0, result.stderr || result.stdout);
  assert.doesNotMatch(result.stderr, /ENOENT|scandir/);
});
