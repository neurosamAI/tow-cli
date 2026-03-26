#!/usr/bin/env node
/**
 * Tow CLI installer for npm
 * by neurosam.AI — https://neurosam.ai
 *
 * Downloads the appropriate binary for the current platform.
 */

const { execSync } = require("child_process");
const fs = require("fs");
const path = require("path");
const https = require("https");

const VERSION = require("./package.json").version;
const BASE_URL = `https://github.com/neurosamAI/tow-cli/releases/download/v${VERSION}`;

function getPlatform() {
  const platform = process.platform;
  const arch = process.arch;

  const platformMap = {
    darwin: "darwin",
    linux: "linux",
  };

  const archMap = {
    x64: "amd64",
    arm64: "arm64",
  };

  const os = platformMap[platform];
  const cpu = archMap[arch];

  if (!os || !cpu) {
    console.error(`Unsupported platform: ${platform}-${arch}`);
    console.error("Tow supports: macOS (amd64/arm64), Linux (amd64/arm64)");
    console.error("Install via Go instead: go install github.com/neurosamAI/tow-cli/cmd/tow@latest");
    process.exit(1);
  }

  return { os, cpu };
}

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    https.get(url, (response) => {
      if (response.statusCode === 302 || response.statusCode === 301) {
        download(response.headers.location, dest).then(resolve).catch(reject);
        return;
      }
      if (response.statusCode !== 200) {
        reject(new Error(`Download failed: HTTP ${response.statusCode}`));
        return;
      }
      response.pipe(file);
      file.on("finish", () => {
        file.close();
        resolve();
      });
    }).on("error", reject);
  });
}

async function main() {
  const { os, cpu } = getPlatform();
  const binaryName = `tow-${os}-${cpu}`;
  const url = `${BASE_URL}/${binaryName}`;
  const binDir = path.join(__dirname, "bin");
  const binPath = path.join(binDir, "tow");

  console.log(`Downloading Tow v${VERSION} for ${os}/${cpu}...`);

  if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true });
  }

  try {
    await download(url, binPath);
    fs.chmodSync(binPath, 0o755);
    console.log(`Tow v${VERSION} installed successfully!`);
    console.log(`Binary: ${binPath}`);
  } catch (err) {
    console.error(`Failed to download Tow: ${err.message}`);
    console.error(`URL: ${url}`);
    console.error("Install via Go instead: go install github.com/neurosamAI/tow-cli/cmd/tow@latest");
    process.exit(1);
  }
}

main();
