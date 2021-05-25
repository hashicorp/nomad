#!/usr/bin/env node
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const git_1 = require("../ingest/git");
const manifest_1 = require("../manifest");
async function main() {
    // TODO: can safely, automatically infer the product name?
    const [, , version] = process.argv;
    const manifest = await manifest_1.loadVersionManifest();
    const foundVersionIndex = manifest.findIndex((ver) => ver.slug === version);
    if (foundVersionIndex === -1) {
        console.error(`Version ${version} not found, you might need to add it first.`);
        return;
    }
    const foundVersion = manifest[foundVersionIndex];
    let sha;
    await git_1.checkoutRefAndExecute(foundVersion.ref, async () => {
        sha = await git_1.getGitSha();
    });
    manifest.splice(foundVersionIndex, 1, {
        ...foundVersion,
        sha,
    });
    await manifest_1.writeVersionManifest([...manifest]);
    console.log(`Version ${version} updated to ${sha} in version-manifest.json`);
}
main();
