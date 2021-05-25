#!/usr/bin/env node
"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const path_1 = __importDefault(require("path"));
const db_1 = require("../db");
const ingest_1 = require("../ingest");
const git_1 = require("../ingest/git");
const manifest_1 = require("../manifest");
const util_1 = require("../util");
// TODO: remove after done with local testing
require('dotenv').config({ path: path_1.default.join(process.cwd(), '.env.local') });
const isDryRun = Boolean(process.env.DRY_RUN);
/**
 * Executed on version-manifest change. Determines which versions need to be
 * uploaded/removed based on the diff between the manifest and the version metadata in the DB
 */
async function main() {
    const [, , product, directory] = process.argv;
    // load version metadata
    const versionsMetadata = (await db_1.retrieveDocument(product, 'versionMetadata'));
    util_1.logIf(isDryRun, 'version metadata loaded:', JSON.stringify(versionsMetadata, null, 2));
    // compare with version-manifest
    const manifest = await manifest_1.loadVersionManifest();
    util_1.logIf(isDryRun, 'version manifest loaded:', JSON.stringify(manifest, null, 2));
    // versions not yet in the metadata, or changed, are to be uploaded
    const versionsToIngest = manifest.filter((version) => !versionsMetadata.versions.find((v) => v.slug === version.slug && v.ref === version.ref));
    for (const version of versionsToIngest) {
        const metadata = versionsMetadata.versions.find((v) => v.slug === version.slug);
        console.log(`Ingesting version: ${version.display}...`);
        util_1.logIf(isDryRun, 'version metadata:', JSON.stringify(metadata, null, 2));
        util_1.logIf(isDryRun, `Checking out ref ${version.ref} and running ingest in ${directory}`);
        await git_1.checkoutRefAndExecute(version.ref, async () => {
            let mdxTransforms, navDataTransforms;
            const sha = await git_1.getGitSha();
            const ingestOpts = {
                product,
                directory: metadata?.directory ?? directory,
                version: { ...metadata, ...version },
                sha,
                isLatestVersion: metadata?.isLatest,
            };
            util_1.logIf(isDryRun, 'calling ingestDocumentationVersion with options:', JSON.stringify(ingestOpts, null, 2));
            if (!isDryRun) {
                ;
                ({
                    mdxTransforms,
                    navDataTransforms,
                } = await ingest_1.ingestDocumentationVersion(ingestOpts));
            }
            if (metadata) {
                metadata.display = version.display;
                metadata.ref = version.ref;
                metadata.sha = sha;
                metadata.directory = directory;
                metadata.mdxTransforms = mdxTransforms;
                metadata.navDataTransforms = navDataTransforms;
            }
            else {
                versionsMetadata.versions.push({
                    ...version,
                    sha,
                    directory,
                    mdxTransforms,
                    navDataTransforms,
                });
            }
        });
    }
    const slugsInManifest = manifest.map((v) => v.slug);
    // filter out any versions no longer in the manifest
    versionsMetadata.versions = versionsMetadata.versions.filter((v) => {
        const isVersionInManifest = slugsInManifest.includes(v.slug);
        util_1.logIf(isDryRun && !isVersionInManifest, `removing version with ref ${v.ref} from version metadata`);
        return isVersionInManifest;
    });
    // update metadata
    util_1.logIf(isDryRun, 'Writing versionMetadata:', JSON.stringify(versionsMetadata, null, 2));
    if (!isDryRun) {
        // writeDocument(versionsMetadata)
    }
}
main();
