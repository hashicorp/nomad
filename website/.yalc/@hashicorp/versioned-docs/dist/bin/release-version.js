#!/usr/bin/env node
"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const path_1 = __importDefault(require("path"));
const manifest_1 = require("../manifest");
const db_1 = require("../db");
const ingest_1 = require("../ingest");
const git_1 = require("../ingest/git");
const util_1 = require("../util");
// TODO: remove after done with local testing
require('dotenv').config({ path: path_1.default.join(process.cwd(), '.env.local') });
const isDryRun = Boolean(process.env.DRY_RUN);
/**
 * Executes on product release, derives the new version from the ref and creates a new entry
 * in the version-manifest. Kicks-off ingest for the new version as latest, re-ingest the previously
 * latest version to apply necessary transforms
 */
async function main() {
    // TODO: can safely, automatically infer the product name?
    const [, , ref, sha, product, directory] = process.argv;
    // assuming refs/tag/{version}
    // TODO: derive the correct branch name
    const version = ref.split('/')[2];
    const manifest = await manifest_1.loadVersionManifest();
    const newVersion = {
        ref,
        slug: version,
        display: version,
    };
    const versionsMetadata = (await db_1.retrieveDocument(product, 'versionMetadata'));
    // TODO: determine if we should create a new entry or update the existing one
    console.log(`Ingesting version: ${newVersion.slug}...`);
    // re-ingest the previously latest version, to apply the additional transforms
    const previousLatest = versionsMetadata.versions.find((v) => v.isLatest);
    if (previousLatest) {
        console.log(`Re-ingesting version: ${previousLatest.slug}...`);
        const previousIngestOpts = {
            product,
            directory: previousLatest.directory,
            version: previousLatest,
            sha: previousLatest.sha,
        };
        util_1.logIf(isDryRun, 'calling ingestDocumentationVersion with options:', JSON.stringify(previousIngestOpts, null, 2));
        let mdxTransforms, navDataTransforms;
        if (!isDryRun) {
            await git_1.checkoutRefAndExecute(previousLatest?.ref, async () => {
                ;
                ({
                    mdxTransforms,
                    navDataTransforms,
                } = await ingest_1.ingestDocumentationVersion(previousIngestOpts));
            });
        }
        previousLatest.isLatest = false;
        previousLatest.mdxTransforms = mdxTransforms;
        previousLatest.navDataTransforms = navDataTransforms;
    }
    const ingestOpts = {
        product,
        directory,
        version: newVersion,
        sha,
        isLatestVersion: true,
    };
    util_1.logIf(isDryRun, 'calling ingestDocumentationVersion with options:', JSON.stringify(ingestOpts, null, 2));
    if (!isDryRun) {
        await ingest_1.ingestDocumentationVersion(ingestOpts);
    }
    versionsMetadata.versions = [
        { ...newVersion, isLatest: true, directory, sha },
        ...versionsMetadata.versions,
    ];
    util_1.logIf(isDryRun, 'Writing versionMetadata:', JSON.stringify(versionsMetadata, null, 2));
    util_1.logIf(isDryRun, 'Writing version-manifest:', JSON.stringify(manifest, null, 2));
    if (!isDryRun) {
        await db_1.writeDocument(versionsMetadata);
        await manifest_1.writeVersionManifest([newVersion, ...manifest]);
    }
}
main();
