#!/usr/bin/env node
/**
 * Next 16 defaults production `next build` to Turbopack, which does not emit
 * `output: "standalone"` artifacts. Deploy needs `.next/standalone/.../server.js`,
 * so when STANDALONE=true we run `next build --webpack`.
 */
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.dirname(fileURLToPath(import.meta.url));
// fumadocs-mdx@12 is incompatible with Next 16 Turbopack; always use webpack.
const pnpmArgs = ["exec", "next", "build", "--webpack"];

const result = spawnSync("pnpm", pnpmArgs, {
  stdio: "inherit",
  cwd: root,
  shell: process.platform === "win32",
});
process.exit(result.status ?? 1);
