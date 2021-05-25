"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.transformRewriteStaticAssets = void 0;
const path_1 = __importDefault(require("path"));
const fs_1 = __importDefault(require("fs"));
const remark_1 = __importDefault(require("remark"));
const remark_mdx_1 = __importDefault(require("remark-mdx"));
const unist_util_flatmap_1 = __importDefault(require("unist-util-flatmap"));
const unist_util_is_1 = __importDefault(require("unist-util-is"));
async function getStaticAssetBasePaths(cwd) {
    const publicDir = path_1.default.join(cwd, 'public');
    return (await fs_1.default.promises.readdir(publicDir)).map((dir) => `/${dir}`);
}
const rewriteStaticAssetsPlugin = ({ version, basePaths, }) => {
    return function transformer(tree) {
        return unist_util_flatmap_1.default(tree, (node) => {
            if (!unist_util_is_1.default(node, 'image') &&
                !unist_util_is_1.default(node, 'link') &&
                !unist_util_is_1.default(node, 'definition'))
                return [node];
            if (path_1.default.isAbsolute(node.url) &&
                basePaths.some((basePath) => node.url.startsWith(basePath))) {
                node.url = `/api/versioned-asset${node.url}?version=${version}`;
            }
            return [node];
        });
    };
};
exports.transformRewriteStaticAssets = {
    id: 'rewrite-static-assets',
    async transformer(document, ctx) {
        const staticAssetBasePaths = await getStaticAssetBasePaths(ctx.cwd);
        const contents = await remark_1.default()
            .use(remark_mdx_1.default)
            .use(rewriteStaticAssetsPlugin, {
            version: ctx.version.ref,
            basePaths: staticAssetBasePaths,
        })
            .process(document.markdownSource);
        document.markdownSource = String(contents);
        return document;
    },
};
