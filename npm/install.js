#!/usr/bin/env node

"use strict";

const { execFileSync } = require("child_process");
const crypto = require("crypto");
const fs = require("fs");
const https = require("https");
const os = require("os");
const path = require("path");
const { mkdirSync } = fs;

const VERSION = require("./package.json").version;
const REPO = "revkeen/cli";
const CHECKSUMS_PATH = path.join(__dirname, "checksums.json");

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

// The expected SHA-256 of every archive this version downloads, written at
// release time from GoReleaser's checksums.txt. npm's own integrity hash covers
// this file, so a release asset replaced after publication no longer installs.
// Missing or mismatched is fatal: there is no unverified install path.
function expectedDigest(archiveName) {
  let manifest;
  try {
    manifest = JSON.parse(fs.readFileSync(CHECKSUMS_PATH, "utf8"));
  } catch (err) {
    throw new Error(
      `Cannot read the checksum manifest at ${CHECKSUMS_PATH} (${err.message}). ` +
        "A published @revkeen/cli-binary always ships one; refusing to install an unverified binary."
    );
  }

  if (manifest.schemaVersion !== 1) {
    throw new Error(`Unsupported checksum manifest schemaVersion: ${manifest.schemaVersion}`);
  }
  if (manifest.algorithm !== "sha256") {
    throw new Error(`Unsupported checksum algorithm: ${manifest.algorithm}`);
  }
  if (manifest.version !== VERSION) {
    throw new Error(
      `Checksum manifest is for v${manifest.version} but this package is v${VERSION}.`
    );
  }

  const digest = manifest.archives?.[archiveName];
  if (typeof digest !== "string" || !/^[0-9a-f]{64}$/.test(digest)) {
    throw new Error(`Checksum manifest has no valid sha256 for ${archiveName}`);
  }
  return digest;
}

function assertHttps(url) {
  if (!/^https:\/\//i.test(url)) {
    throw new Error(`Refusing a non-HTTPS URL: ${url}`);
  }
}

function downloadFile(url) {
  return new Promise((resolve, reject) => {
    const follow = (target, redirects) => {
      if (redirects > 5) {
        return reject(new Error("Too many redirects"));
      }

      try {
        // A redirect must not downgrade the transport. GitHub redirects release
        // downloads to objects.githubusercontent.com over HTTPS; anything else
        // is refused rather than followed over plain HTTP.
        assertHttps(target);
      } catch (err) {
        return reject(err);
      }

      https
        .get(target, { headers: { "User-Agent": "revkeen-cli-npm" } }, (res) => {
          if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
            // Drain the redirect body: an unread response keeps the TLS socket
            // open, and node then waits ~30 s before the process can exit.
            res.resume();
            return follow(new URL(res.headers.location, target).toString(), redirects + 1);
          }
          if (res.statusCode !== 200) {
            res.resume();
            return reject(new Error(`Download failed: HTTP ${res.statusCode} from ${target}`));
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

function verifyDigest(buffer, archiveName) {
  const expected = expectedDigest(archiveName);
  const actual = crypto.createHash("sha256").update(buffer).digest("hex");
  if (actual !== expected) {
    throw new Error(
      `Checksum mismatch for ${archiveName}: expected ${expected}, got ${actual}. ` +
        "The downloaded archive is not the one this package was published against; nothing was extracted."
    );
  }
}

// execFile with an argument array: no shell, so no path or filename is ever
// parsed as a command. The temp directory is created with mkdtemp rather than a
// guessable Date.now() name.
function withTempArchive(buffer, suffix, run) {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "revkeen-"));
  const tmpFile = path.join(tmpDir, `revkeen.${suffix}`);
  fs.writeFileSync(tmpFile, buffer, { mode: 0o600 });
  try {
    run(tmpFile);
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

function extractTarGz(buffer, destDir) {
  withTempArchive(buffer, "tar.gz", (tmpFile) => {
    execFileSync("tar", ["-xzf", tmpFile, "-C", destDir], { stdio: "pipe" });
  });
}

function extractZip(buffer, destDir) {
  withTempArchive(buffer, "zip", (tmpFile) => {
    if (process.platform === "win32") {
      // The paths travel in the environment, so PowerShell never parses them.
      execFileSync(
        "powershell",
        [
          "-NoProfile",
          "-NonInteractive",
          "-Command",
          "Expand-Archive -LiteralPath $env:REVKEEN_ARCHIVE -DestinationPath $env:REVKEEN_DEST -Force",
        ],
        {
          stdio: "pipe",
          env: { ...process.env, REVKEEN_ARCHIVE: tmpFile, REVKEEN_DEST: destDir },
        }
      );
    } else {
      execFileSync("unzip", ["-o", tmpFile, "-d", destDir], { stdio: "pipe" });
    }
  });
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

  // Read the manifest before the download, so a package that cannot verify
  // fails before it fetches anything.
  expectedDigest(archiveName);

  const url = `https://github.com/${REPO}/releases/download/v${VERSION}/${archiveName}`;
  console.log(`Downloading revkeen v${VERSION} for ${process.platform}/${process.arch}...`);
  console.log(`  ${url}`);

  const buffer = await downloadFile(url);
  verifyDigest(buffer, archiveName);
  console.log(`  sha256 verified against checksums.json`);

  mkdirSync(binDir, { recursive: true });

  if (archiveName.endsWith(".zip")) {
    extractZip(buffer, binDir);
  } else {
    extractTarGz(buffer, binDir);
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
