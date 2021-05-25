#!/usr/bin/env node
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const manifest_1 = require("../manifest");
async function main() {
    const [, , display] = process.argv;
    const manifest = await manifest_1.loadVersionManifest();
    const versionIndex = manifest.findIndex((version) => version.display === display);
    if (versionIndex === -1) {
        console.error('Version not found in the manifest');
        return;
    }
    const newManifest = [...manifest];
    newManifest.splice(versionIndex, 1);
    await manifest_1.writeVersionManifest(newManifest);
    console.log(`Version ${display} removed from version-manifest.json`);
}
main();
