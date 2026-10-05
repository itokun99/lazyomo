#!/usr/bin/env node
// Minimal launcher for the lazyomo Go binary (no dependencies, stdlib only).
//
// Sibling packaged path (written by scripts/install.js:getInstallPath()):
//   <this-dir>/lazyomo[.exe]  (i.e. bin/lazyomo, bin/lazyomo.exe on win32)
// Lookup order:
//   1) sibling packaged binary above,
//   2) `lazyomo` on PATH.
// Spawns with the same argv and propagates the exit code.

const fs = require("fs");
const path = require("path");
const { spawn } = require("child_process");

const sibling = path.join(
  __dirname,
  process.platform === "win32" ? "lazyomo.exe" : "lazyomo"
);

function noBinary() {
  console.error("lazyomo binary not found. Install with: npm i -g @itokun99/lazyomo, or go install github.com/itokun99/lazyomo/cmd/lazyomo@latest");
  process.exit(1);
}

function run(cmd, args) {
  const child = spawn(cmd, args, { stdio: "inherit" });
  child.on("error", (err) => {
    if (err && err.code === "ENOENT") {
      if (cmd !== "lazyomo") {
        run("lazyomo", args);
      } else {
        noBinary();
      }
    } else {
      console.error(String((err && err.message) || err));
      process.exit(1);
    }
  });
  child.on("close", (code, signal) => {
    if (signal) {
      try {
        process.kill(process.pid, signal);
      } catch (_) {
        process.exit(1);
      }
      return;
    }
    process.exit(code == null ? 1 : code);
  });
}

const args = process.argv.slice(2);
let target;
try {
  target = fs.existsSync(sibling) ? sibling : "lazyomo";
} catch (_) {
  target = "lazyomo";
}
run(target, args);
