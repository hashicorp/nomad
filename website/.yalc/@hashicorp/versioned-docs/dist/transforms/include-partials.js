"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.transformIncludePartials = void 0;
const path_1 = __importDefault(require("path"));
const remark_1 = __importDefault(require("remark"));
const remark_mdx_1 = __importDefault(require("remark-mdx"));
const remark_plugins_1 = require("@hashicorp/remark-plugins");
exports.transformIncludePartials = {
    id: 'include-partials',
    async transformer(document, ctx) {
        const partialDir = path_1.default.join(ctx.cwd, ctx.rootDir, 'partials');
        const contents = await remark_1.default()
            .use(remark_mdx_1.default)
            .use(remark_plugins_1.includeMarkdown, { resolveFrom: partialDir })
            .process(document.markdownSource);
        document.markdownSource = String(contents);
        return document;
    },
};
