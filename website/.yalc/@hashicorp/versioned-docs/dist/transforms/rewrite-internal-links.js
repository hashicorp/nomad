"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.transformRewriteInternalLinks = void 0;
const remark_1 = __importDefault(require("remark"));
const remark_mdx_1 = __importDefault(require("remark-mdx"));
const unist_util_flatmap_1 = __importDefault(require("unist-util-flatmap"));
const unist_util_is_1 = __importDefault(require("unist-util-is"));
const rewriteInternalLinksPlugin = ({ version, basePaths }) => {
    // This pattern matches both absolute links and relative links:
    // - ../../api-docs/command (true)
    // - /docs/intro (true)
    const isLinkToRewritePattern = new RegExp(`^(((\\.+\\/)*)|\\/)(${basePaths.join('|')})`);
    return function transformer(tree) {
        return unist_util_flatmap_1.default(tree, (node) => {
            if (!unist_util_is_1.default(node, 'link') && !unist_util_is_1.default(node, 'definition'))
                return [node];
            // internal link, slap the version in there
            if (isLinkToRewritePattern.test(node.url)) {
                const pathParts = node.url.split('/');
                const basePathIndex = pathParts.findIndex((part) => basePaths.includes(part));
                pathParts.splice(basePathIndex + 1, 0, version);
                node.url = pathParts.join('/');
            }
            return [node];
        });
    };
};
exports.transformRewriteInternalLinks = {
    id: 'rewrite-internal-links-v2',
    async transformer(document, ctx) {
        const contents = await remark_1.default()
            .use(remark_mdx_1.default)
            .use(rewriteInternalLinksPlugin, {
            version: document.version,
            basePaths: ctx.basePaths,
        })
            .process(document.markdownSource);
        document.markdownSource = String(contents);
        return document;
    },
};
