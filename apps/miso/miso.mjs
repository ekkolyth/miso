#!/usr/bin/env node

import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

function resolveBinary() {
  const platform = process.platform;
  const arch = process.arch;

  /** @type {Record<string, string>} */
  const targets = {
    'darwin-x64': 'miso-darwin-amd64',
    'darwin-arm64': 'miso-darwin-arm64',
    'linux-x64': 'miso-linux-amd64',
    'linux-arm64': 'miso-linux-arm64',
  };

  const key = `${platform}-${arch}`;
  const binaryName = targets[key];

  if (!binaryName) {
    console.error(
      `Unsupported platform/arch: ${platform}/${arch}. ` +
        'Prebuilt binaries are only provided for darwin-x64, darwin-arm64, linux-x64, and linux-arm64.',
    );
    process.exit(1);
  }

  return join(__dirname, 'bin', binaryName);
}

const binPath = resolveBinary();

const result = spawnSync(binPath, process.argv.slice(2), {
  stdio: 'inherit',
});

if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}

process.exit(result.status ?? 0);

