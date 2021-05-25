"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.getVersionFromPath = exports.loadVersionedDocument = exports.loadVersionedNavData = exports.loadVersionList = exports.makeServeStaticAssets = void 0;
const db_1 = require("../db");
const util_1 = require("../util");
Object.defineProperty(exports, "getVersionFromPath", { enumerable: true, get: function () { return util_1.getVersionFromPath; } });
var serve_static_asset_1 = require("./serve-static-asset");
Object.defineProperty(exports, "makeServeStaticAssets", { enumerable: true, get: function () { return serve_static_asset_1.makeServeStaticAssets; } });
/**
 * Returns a list of versions for use within the application, in the version selector for example.
 */
async function loadVersionList(product) {
    // const manifest = await loadVersionManifest()
    const versionsMetadata = (await db_1.retrieveDocument(product, 'versionMetadata'));
    if (!versionsMetadata)
        return [];
    return versionsMetadata.versions.map((version) => version.isLatest
        ? {
            label: `${version.display} (latest)`,
            name: 'latest',
        }
        : {
            name: version.slug,
            label: version.display,
        });
}
exports.loadVersionList = loadVersionList;
// TODO: implement
async function loadVersionedNavData(product, basePath, version) {
    const document = await db_1.retrieveDocument(product, `nav-data/${basePath}/${version}`);
    return document;
}
exports.loadVersionedNavData = loadVersionedNavData;
// TODO: probably just put the implementation of retrieveDocument here
async function loadVersionedDocument(product, fullPath) {
    const document = await db_1.retrieveDocument(product, fullPath);
    return document;
}
exports.loadVersionedDocument = loadVersionedDocument;
