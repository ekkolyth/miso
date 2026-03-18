#!/usr/bin/env node

/**
 * Prints a one-time hint after `npm install -g @ekkolyth/miso`.
 * Does nothing that can break — no file writes, no side effects.
 */

const reset = "\x1b[0m";
const bold = "\x1b[1m";
const cyan = "\x1b[36m";
const yellow = "\x1b[33m";

// Suppress the hint when npm is running in a CI environment or
// when the install is happening as a dependency of another package
// (i.e. not a direct / top-level install).
if (process.env.CI || process.env.npm_config_global !== "true") {
	process.exit(0);
}

console.log(`
${bold}${cyan}miso${reset} was installed successfully.

${yellow}To enable shell tab-completion, run:${reset}

  ${bold}node "$(npm root -g)/@ekkolyth/miso/scripts/completion.mjs"${reset}

This only needs to be done once. Restart your shell afterwards.
`);
