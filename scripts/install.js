#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const https = require('https');

const PLATFORM = process.platform;
const ARCH = process.arch;

const PLATFORM_MAP = {
  'darwin': 'darwin',
  'linux': 'linux',
  'win32': 'windows'
};

const ARCH_MAP = {
  'x64': 'amd64',
  'arm64': 'arm64'
};

const REPO = 'itokun99/lazyomo';
// Single source of truth for the release tag: package.json version.
const VERSION = 'v' + require('../package.json').version;

// Asset naming contract (shared with .github/workflows/release.yml and
// Formula/lazyomo.rb): lazyomo-<goos>-<goarch>[.exe on windows].
// Examples: lazyomo-darwin-amd64, lazyomo-darwin-arm64,
// lazyomo-linux-amd64, lazyomo-linux-arm64,
// lazyomo-windows-amd64.exe, lazyomo-windows-arm64.exe
function getBinaryName() {
  const platform = PLATFORM_MAP[PLATFORM];
  const arch = ARCH_MAP[ARCH];

  if (!platform || !arch) {
    console.error(`Unsupported platform: ${PLATFORM}-${ARCH}`);
    process.exit(1);
  }

  const ext = PLATFORM === 'win32' ? '.exe' : '';
  return `lazyomo-${platform}-${arch}${ext}`;
}

function getDownloadUrl() {
  const binary = getBinaryName();
  return `https://github.com/${REPO}/releases/download/${VERSION}/${binary}`;
}

// Packaged sibling path also used by bin/lazyomo.js launcher lookup:
// bin/lazyomo[.exe] next to the launcher.
function getInstallPath() {
  const binDir = path.join(__dirname, '..', 'bin');
  const ext = PLATFORM === 'win32' ? '.exe' : '';
  return path.join(binDir, `lazyomo${ext}`);
}

async function download(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    https.get(url, (response) => {
      if (response.statusCode === 302 || response.statusCode === 301) {
        // Follow redirect
        download(response.headers.location, dest).then(resolve).catch(reject);
        return;
      }
      if (response.statusCode !== 200) {
        reject(new Error(`Download failed: ${response.statusCode}`));
        return;
      }
      response.pipe(file);
      file.on('finish', () => {
        file.close();
        resolve();
      });
    }).on('error', (err) => {
      fs.unlink(dest, () => {});
      reject(err);
    });
  });
}

async function main() {
  const url = getDownloadUrl();
  const dest = getInstallPath();

  console.log(`Downloading lazyomo for ${PLATFORM}-${ARCH}...`);
  console.log(`URL: ${url}`);

  try {
    await download(url, dest);
    fs.chmodSync(dest, '755');
    console.log('Installation complete!');
  } catch (err) {
    console.error('Installation failed:', err.message);
    console.error('');
    console.error('Please install manually:');
    console.error('  go install github.com/itokun99/lazyomo/cmd/lazyomo@latest');
    // Don't fail the install — allow manual install
    process.exit(0);
  }
}

main();
