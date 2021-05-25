"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.applyTransformToDocument = exports.LATEST_TRANSFORMS = exports.BASE_TRANSFORMS = void 0;
const extract_frontmatter_1 = require("./extract-frontmatter");
const include_partials_1 = require("./include-partials");
const paragraph_custom_alerts_1 = require("./paragraph-custom-alerts");
const rewrite_internal_links_1 = require("./rewrite-internal-links");
const rewrite_static_assets_1 = require("./rewrite-static-assets");
const rewrite_internal_redirects_1 = require("./rewrite-internal-redirects");
const transforms = {
    [extract_frontmatter_1.transformExtractFrontmatter.id]: extract_frontmatter_1.transformExtractFrontmatter,
    [include_partials_1.transformIncludePartials.id]: include_partials_1.transformIncludePartials,
    [rewrite_internal_links_1.transformRewriteInternalLinks.id]: rewrite_internal_links_1.transformRewriteInternalLinks,
    [paragraph_custom_alerts_1.transformParagraphCustomAlerts.id]: paragraph_custom_alerts_1.transformParagraphCustomAlerts,
    [rewrite_static_assets_1.transformRewriteStaticAssets.id]: rewrite_static_assets_1.transformRewriteStaticAssets,
    [rewrite_internal_redirects_1.transformRewriteInternalRedirects.id]: rewrite_internal_redirects_1.transformRewriteInternalRedirects,
    'test-transform': {
        id: 'test-transform',
        async transformer(document) {
            return document;
        },
    },
};
// Default transforms applied to all documents, order is respected!
exports.BASE_TRANSFORMS = [
    extract_frontmatter_1.transformExtractFrontmatter.id,
    include_partials_1.transformIncludePartials.id,
    paragraph_custom_alerts_1.transformParagraphCustomAlerts.id,
    rewrite_internal_redirects_1.transformRewriteInternalRedirects.id,
    rewrite_internal_links_1.transformRewriteInternalLinks.id,
    rewrite_static_assets_1.transformRewriteStaticAssets.id,
];
// Transforms applied to the latest version only
exports.LATEST_TRANSFORMS = [extract_frontmatter_1.transformExtractFrontmatter.id];
/**
 * Applies the specified transform to the given document
 *
 * @param document Document to apply the transform against
 * @param transformArg Transform ID or Transform object
 * @param rootDir Directory where the content is being pulled from
 */
async function applyTransformToDocument(document, transformArg, ctx) {
    let transform = typeof transformArg === 'string' ? transforms[transformArg] : transformArg;
    const newDocument = await transform.transformer(document, {
        cwd: process.cwd(),
        ...ctx,
    });
    newDocument.mdxTransforms.push(transform.id);
    return newDocument;
}
exports.applyTransformToDocument = applyTransformToDocument;
