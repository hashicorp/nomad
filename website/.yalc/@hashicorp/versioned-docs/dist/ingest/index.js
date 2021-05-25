"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.ingestDocumentationVersion = exports.addVersionToNavDataLinks = exports.extractNavData = exports.extractDocuments = void 0;
/**
 * Ingest encompasses the ETL process to fully upload the documentation data for a project's specific version
 */
const readdirp_1 = __importDefault(require("readdirp"));
const path_1 = __importDefault(require("path"));
const fs_1 = __importDefault(require("fs"));
const util_1 = require("../util");
const transforms_1 = require("../transforms");
const migrate_nav_data_to_json_1 = require("./migrate-nav-data-to-json");
/**
 * Extracts documents from the specified directory
 */
async function extractDocuments(directory) {
    let result = [];
    const dirPath = path_1.default.join(process.cwd(), directory);
    for await (const entry of readdirp_1.default(path_1.default.join(dirPath), {
        directoryFilter: ['!partials'],
        fileFilter: ['*.md', '*.mdx'],
    })) {
        const fileContents = await fs_1.default.promises.readFile(entry.fullPath, {
            encoding: 'utf-8',
        });
        result.push({
            content: fileContents,
            filePath: path_1.default.join(directory, entry.path),
        });
    }
    return result;
}
exports.extractDocuments = extractDocuments;
/**
 * Extract navigation data from the old js format, or the new json format
 *
 * TODO: we are reading all of the content files as a result of this for frontmatter.
 *       Optimize so we are only reading the files once throughout the entire ETL process.
 */
async function extractNavData(directory, version) {
    let allNavData = [];
    // Read the subdirectories within directory
    const dirs = (await fs_1.default.promises.readdir(path_1.default.join(process.cwd(), directory))).reduce((res, dir) => {
        const navDataFile = version?.navData?.[dir];
        // if we have a mapping specified in the manifest, use that
        if (navDataFile) {
            res.push([dir, path_1.default.join(process.cwd(), 'data', navDataFile)]);
        }
        else {
            // otherwise, try and infer the name of the nav data file
            const pathToJsonNavData = path_1.default.join(process.cwd(), 'data', `${dir}-nav-data.json`);
            if (fs_1.default.existsSync(pathToJsonNavData)) {
                res.push([dir, pathToJsonNavData]);
            }
        }
        return res;
    }, []);
    dirs.forEach(async ([dir, pathToNavData]) => {
        // If we find the nav data in the new JSON format, return that without modification
        if (pathToNavData.endsWith('.json')) {
            const navData = JSON.parse(await fs_1.default.promises.readFile(pathToNavData, { encoding: 'utf-8' }));
            allNavData.push([dir, navData]);
            return;
        }
        // Otherwise, grab the js format and transform it to the new JSON format
        const navigationJsData = await migrate_nav_data_to_json_1.loadJSNavData(pathToNavData);
        const { navData } = await migrate_nav_data_to_json_1.getMigratedNavData(navigationJsData, path_1.default.join(process.cwd(), directory, dir));
        allNavData.push([dir, navData]);
    });
    return allNavData;
}
exports.extractNavData = extractNavData;
/**
 * Adds the version segment to nav data links
 */
function addVersionToNavDataLinks(version, navData) {
    return navData.map((node) => {
        if ('routes' in node) {
            node.routes = addVersionToNavDataLinks(version, node.routes);
            return node;
        }
        if ('path' in node) {
            node.path = `${version}${node.path.startsWith('/') ? node.path : `/${node.path}`}`;
        }
        return node;
    });
}
exports.addVersionToNavDataLinks = addVersionToNavDataLinks;
/**
 * Ingest a complete version of a product's documentation, including all pages as well as navigation data
 */
async function ingestDocumentationVersion({ product, version, directory, sha, isLatestVersion, }) {
    // extract
    const [rawDocuments, navData] = await Promise.all([
        extractDocuments(directory),
        extractNavData(directory, version),
    ]);
    // transform
    const structuredDocuments = rawDocuments.map((doc) => {
        const { fullPath, subpath } = util_1.makeVersionedPath({
            version: version.slug,
            filePath: doc.filePath,
        });
        return {
            sha,
            version: version.slug,
            product,
            subpath,
            fullPath,
            markdownSource: doc.content,
            mdxTransforms: [],
        };
    });
    const structuredNavData = navData.map(([dir, curNavData]) => ({
        sha,
        version: version.slug,
        product,
        subpath: dir,
        fullPath: `nav-data/${dir}/${version.slug}`,
        navData: isLatestVersion
            ? curNavData
            : addVersionToNavDataLinks(version.slug, curNavData),
    }));
    const transformsToApply = isLatestVersion
        ? transforms_1.LATEST_TRANSFORMS
        : transforms_1.BASE_TRANSFORMS;
    const basePaths = navData.map(([dir]) => dir);
    // Apply Transforms
    await Promise.all(structuredDocuments.map(async (doc) => {
        for (const transform of transformsToApply) {
            await transforms_1.applyTransformToDocument(doc, transform, {
                rootDir: directory,
                basePaths,
                version,
            });
        }
    }));
    // load
    // await Promise.all([
    //   ...structuredDocuments.map((doc) => writeDocument(doc)),
    //   ...structuredNavData.map((doc) => writeDocument(doc)),
    // ])
    // structuredDocuments.forEach((doc) =>
    //   console.log(doc.fullPath, doc.mdxTransforms)
    // )
    structuredNavData.forEach((doc) => console.log(doc.fullPath));
    return {
        mdxTransforms: transformsToApply,
        navDataTransforms: ['add-version'],
    };
}
exports.ingestDocumentationVersion = ingestDocumentationVersion;
