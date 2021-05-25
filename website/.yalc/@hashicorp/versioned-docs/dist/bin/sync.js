#!/usr/bin/env node
"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const path_1 = __importDefault(require("path"));
const db_1 = require("../db");
const ingest_1 = require("../ingest");
const util_1 = require("../util");
// TODO: remove after done with local testing
require('dotenv').config({ path: path_1.default.join(process.cwd(), '.env.local') });
const isDryRun = Boolean(process.env.DRY_RUN);
/**
 * Executed on push to a release branch
 * If entry found in the version metadata for the corresponding ref, update the content
 * from the ref's HEAD
 */
async function main() {
    const [, , ref, sha, product] = process.argv;
    // const cleanRef = ref.split('/').slice(2).join('/')
    // load version metadata
    const versionsMetadata = (await db_1.retrieveDocument(product, 'versionMetadata'));
    // find version matching ref
    const versionIndex = versionsMetadata.versions.findIndex((v) => v.ref === ref);
    // if not found, do nothing
    if (versionIndex === -1) {
        return;
    }
    const version = versionsMetadata.versions[versionIndex];
    console.log(`Syncing version: ${version.display}...`);
    let mdxTransforms, navDataTransforms;
    if (!isDryRun) {
        ;
        ({ mdxTransforms, navDataTransforms } = await ingest_1.ingestDocumentationVersion({
            product,
            directory: version.directory,
            isLatestVersion: version.isLatest,
            version,
            sha,
        }));
    }
    // update the associated metadata properties
    version.sha = sha;
    version.mdxTransforms = mdxTransforms;
    version.navDataTransforms = navDataTransforms;
    util_1.logIf(isDryRun, 'Writing versionMetadata:', JSON.stringify(versionsMetadata, null, 2));
    if (!isDryRun) {
        await db_1.writeDocument(versionsMetadata);
    }
}
main();
