"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const path_1 = __importDefault(require("path"));
const db_1 = require("../db");
const server_1 = require("../server");
// TODO: remove after done with local testing
require('dotenv').config({ path: path_1.default.join(process.cwd(), '.env.local') });
async function main() {
    const fullPath = 'versionMetadata';
    const seed = {
        fullPath,
        product: 'nomad',
        versions: [
            {
                display: 'v1.1.0',
                slug: 'v1.1.0',
                ref: 'release-1.1.0',
                sha: 'f99f1e27bb66bee36a1f3cdf00335e81e93ffff2',
                directory: 'content',
                isLatest: true,
            },
            {
                display: 'v1.0.x',
                slug: 'v1.0.x',
                ref: 'release-1.0.6',
                sha: 'ef7978872369eebbca2c49bc81cf34769d685e24',
                directory: 'content',
                navData: {
                    docs: 'docs-navigation.js',
                    'api-docs': 'api-navigation.js',
                    intro: 'intro-navigation.js',
                },
            },
            {
                display: 'v0.12.x',
                slug: 'v0.12.x',
                ref: 'v0.12.122',
                sha: 'b7ef1c26d322fec47f1207605b8982d49f54c9a7',
                directory: 'pages',
                navData: {
                    docs: 'docs-navigation.js',
                    'api-docs': 'api-navigation.js',
                    intro: 'intro-navigation.js',
                },
            },
        ],
    };
    await db_1.writeDocument(seed);
    const versionList = await server_1.loadVersionList('nomad');
    console.log(versionList);
}
main();
