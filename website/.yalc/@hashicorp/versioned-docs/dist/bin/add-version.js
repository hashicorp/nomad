#!/usr/bin/env node
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const git_1 = require("../ingest/git");
const manifest_1 = require("../manifest");
async function main() {
    // TODO: can safely, automatically infer the product name?
    const [, , version, ref = version, slug = version, display = version,] = process.argv;
    const manifest = await manifest_1.loadVersionManifest();
    if (manifest.find((version) => version.slug === slug)) {
        console.error('Version already exists in the manifest');
        return;
    }
    let sha;
    await git_1.checkoutRefAndExecute(ref, async () => {
        sha = await git_1.getGitSha();
    });
    const newVersion = {
        display,
        slug,
        ref,
        sha,
    };
    await manifest_1.writeVersionManifest([...manifest, newVersion]);
    console.log(`Version ${version} added to version-manifest.json`);
}
main();
