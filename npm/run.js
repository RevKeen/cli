#!/usr/bin/env node

"use strict";

const { execFileSync } = require("child_process");
const path = require("path");
const { existsSync } = require("fs");

const binaryName = process.platform === "win32" ? "revkeen.exe" : "revkeen";
const binaryPath = path.join(__dirname, "bin", binaryName);

// The binary is not published; install.js fetches the right one for this platform
// at postinstall. Absent means postinstall was skipped (npm install
// --ignore-scripts) or it failed, and a raw ENOENT from execFileSync says none of
// that.
if (!existsSync(binaryPath)) {
  console.error(`revkeen: no binary at ${binaryPath}.

It is downloaded when this package is installed, so this usually means install
scripts were skipped (npm install --ignore-scripts) or the download failed.

Fetch it with:  node ${path.join(__dirname, "install.js")}`);
  process.exit(1);
}

try {
  execFileSync(binaryPath, process.argv.slice(2), { stdio: "inherit" });
} catch (err) {
  if (err.status !== undefined) {
    process.exit(err.status);
  }
  console.error(`Failed to run revkeen: ${err.message}`);
  process.exit(1);
}
