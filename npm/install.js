#!/usr/bin/env node

"use strict";

const { execSync } = require("child_process");
const fs = require("fs");
const https = require("https");
const os = require("os");
const path = require("path");
const { createWriteStream, mkdirSync } = fs;

const VERSION = require("./package.json").version;
const REPO = "revkeen/cli";

const PLATFORM_MAP = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows",
};

const ARCH_MAP = {
  x64: "amd64",
  arm64: "arm64",
};

function getArchiveName() {
  const platform = PLATFORM_MAP[process.platform];
  const arch = ARCH_MAP[process.arch];

  if (!platform) {
    throw new Error(`Unsupported platform: ${process.platform}`);
  }
  if (!arch) {
    throw new Error(`Unsupported architecture: ${process.arch}`);
  }
  if (platform === "windows" && arch === "arm64") {
    throw new Error("Windows arm64 is not supported");
  }

  const ext = platform === "windows" ? "zip" : "tar.gz";
  return `revkeen_${platform}_${arch}.${ext}`;
}

function getBinaryName() {
  return process.platform === "win32" ? "revkeen.exe" : "revkeen";
}

function downloadFile(url) {
  return new Promise((resolve, reject) => {
    const follow = (url, redirects) => {
      if (redirects > 5) {
        return reject(new Error("Too many redirects"));
      }

      const proto = url.startsWith("https") ? https : require("http");
      proto
        .get(url, { headers: { "User-Agent": "revkeen-cli-npm" } }, (res) => {
          if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
            return follow(res.headers.location, redirects + 1);
          }
          if (res.statusCode !== 200) {
            return reject(
              new Error(`Download failed: HTTP ${res.statusCode} from ${url}`)
            );
          }
          const chunks = [];
          res.on("data", (chunk) => chunks.push(chunk));
          res.on("end", () => resolve(Buffer.concat(chunks)));
          res.on("error", reject);
        })
        .on("error", reject);
    };
    follow(url, 0);
  });
}

async function extractTarGz(buffer, destDir) {
  const tmpFile = path.join(os.tmpdir(), `revkeen-${Date.now()}.tar.gz`);
  fs.writeFileSync(tmpFile, buffer);
  try {
    execSync(`tar -xzf "${tmpFile}" -C "${destDir}"`, { stdio: "pipe" });
  } finally {
    fs.unlinkSync(tmpFile);
  }
}

async function extractZip(buffer, destDir) {
  const tmpFile = path.join(os.tmpdir(), `revkeen-${Date.now()}.zip`);
  fs.writeFileSync(tmpFile, buffer);
  try {
    // Use PowerShell on Windows, unzip on Unix
    if (process.platform === "win32") {
      execSync(
        `powershell -Command "Expand-Archive -Path '${tmpFile}' -DestinationPath '${destDir}' -Force"`,
        { stdio: "pipe" }
      );
    } else {
      execSync(`unzip -o "${tmpFile}" -d "${destDir}"`, { stdio: "pipe" });
    }
  } finally {
    fs.unlinkSync(tmpFile);
  }
}

async function main() {
  const archiveName = getArchiveName();
  const binaryName = getBinaryName();
  const binDir = path.join(__dirname, "bin");
  const binaryPath = path.join(binDir, binaryName);

  // Skip if binary already exists (e.g. CI caching)
  if (fs.existsSync(binaryPath)) {
    console.log(`revkeen binary already exists at ${binaryPath}`);
    return;
  }

  const url = `https://github.com/${REPO}/releases/download/v${VERSION}/${archiveName}`;
  console.log(`Downloading revkeen v${VERSION} for ${process.platform}/${process.arch}...`);
  console.log(`  ${url}`);

  const buffer = await downloadFile(url);

  mkdirSync(binDir, { recursive: true });

  if (archiveName.endsWith(".zip")) {
    await extractZip(buffer, binDir);
  } else {
    await extractTarGz(buffer, binDir);
  }

  // chmod +x on Unix
  if (process.platform !== "win32") {
    fs.chmodSync(binaryPath, 0o755);
  }

  // Verify the binary exists
  if (!fs.existsSync(binaryPath)) {
    throw new Error(
      `Binary not found at ${binaryPath} after extraction. Archive contents may have unexpected structure.`
    );
  }

  console.log(`Installed revkeen v${VERSION} to ${binaryPath}`);
}

main().catch((err) => {
  // In CI or monorepo installs, the release may not exist yet.
  // Don't fail pnpm install — the binary is only needed by npm consumers.
  if (process.env.CI || process.env.REVKEEN_SKIP_CLI_DOWNLOAD) {
    console.warn(`Skipping revkeen CLI download (CI): ${err.message}`);
    process.exit(0);
  }
  console.error(`Failed to install revkeen: ${err.message}`);
  process.exit(1);
});
