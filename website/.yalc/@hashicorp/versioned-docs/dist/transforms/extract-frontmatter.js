"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.transformExtractFrontmatter = void 0;
const gray_matter_1 = __importDefault(require("gray-matter"));
exports.transformExtractFrontmatter = {
    id: 'extract-frontmatter',
    async transformer(document, ctx) {
        const { content, data } = gray_matter_1.default(document.markdownSource);
        document.markdownSource = content.trim();
        document.metadata = data;
        return document;
    },
};
