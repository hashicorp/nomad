"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.getMigratedNavData = exports.loadJSNavData = void 0;
// TODO: pull this from a shared location, copied from here: https://github.com/hashicorp/mktg-codemods/pull/7/files
const fs_1 = __importDefault(require("fs"));
const path_1 = __importDefault(require("path"));
const gray_matter_1 = __importDefault(require("gray-matter"));
const klaw_sync_1 = __importDefault(require("klaw-sync"));
async function loadJSNavData(pathToFile) {
    const contents = await fs_1.default.promises.readFile(pathToFile, { encoding: 'utf-8' });
    const openingBracketIndex = contents.split('').findIndex((el) => el === '[');
    const navData = Function(`return (${contents.substr(openingBracketIndex)})`)();
    return navData;
}
exports.loadJSNavData = loadJSNavData;
async function getMigratedNavData(navJsData, contentDir) {
    const fileFilter = (f) => path_1.default.extname(f.path) === '.mdx';
    const collectedFrontmatter = await collectFrontmatter(contentDir, fileFilter);
    const navData = convertNavData(navJsData, collectedFrontmatter, [], '');
    return { navData, collectedFrontmatter };
}
exports.getMigratedNavData = getMigratedNavData;
function convertNavData(navJsData, collectedFrontmatter, pathStack, subfolder) {
    const convertedTree = navJsData.map((navJsNode) => {
        // if the node is a string like '-----',
        // we want to render a divider
        const isString = typeof navJsNode === 'string';
        const isDivider = isString && navJsNode.match(/^-+$/);
        if (isDivider)
            return { divider: true };
        // if the node is a string, but not a divider,
        // we want to render a leaf node
        if (isString)
            return convertNavLeaf(navJsNode, collectedFrontmatter, pathStack, subfolder);
        // if the node has an `href` or `title`, it's a direct link
        if (navJsNode.href || navJsNode.title) {
            if (!navJsNode.href || !navJsNode.title) {
                // if a direct link doesn't have both `href` and `title`, we throw an error
                throw new Error(`Direct sidebar links must have both a "href" and "title". Found a direct link with only one of the two:\n\n ${JSON.stringify(navJsNode)}`);
            }
            return { title: navJsNode.title, href: navJsNode.href };
        }
        // Otherwise, we expect the node to be a nested category
        return convertNavCategory(navJsNode, collectedFrontmatter, pathStack, subfolder);
    });
    return convertedTree;
}
function convertNavCategory(navJsNode, collectedFrontmatter, pathStack, subfolder) {
    // Throw an error if the category is invalid
    if (!navJsNode.category && !navJsNode.content) {
        throw new Error(`Nav category nodes must have either a .name or .category property. ${JSON.stringify(navJsNode, null, 2)}`);
    }
    //  Process the navJsNode's nested content into the new format
    const nestedPathStack = pathStack.concat(navJsNode.category);
    const nestedRoutes = navJsNode.content
        ? convertNavData(navJsNode.content, collectedFrontmatter, nestedPathStack, subfolder)
        : [];
    // Then, handle index data, which is a bit more of an undertaking...
    //  First, we try to gather index data for the entry
    //  The path we want for our new format does NOT contain
    // the content subfolder or the file extension (always `.mdx`)
    const pathParts = [navJsNode.category];
    const pathNewFormat = pathStack.concat(pathParts).join('/');
    // The path we need to match from the older format
    // includes the content subfolder as well as the file extension
    const pathFromSubfolder = `${pathNewFormat}/index.mdx`;
    const pathToMatch = path_1.default.join(subfolder, pathFromSubfolder);
    // Try to find the corresponding index page resource
    const matchedFrontmatter = collectedFrontmatter.filter((resource) => {
        return resource.__resourcePath === pathToMatch;
    })[0];
    const fmTitle = matchedFrontmatter
        ? matchedFrontmatter.sidebar_title || matchedFrontmatter.page_title
        : false;
    if (!fmTitle && !navJsNode.name) {
        throw new Error(`Nav category nodes must have either an index file with a sidebar_title or page_title in the frontmatter, or a .name property.`);
    }
    const title = fmTitle || navJsNode.name;
    // Set up an index page entry, if applicable
    const indexRoute = matchedFrontmatter
        ? {
            title: 'Overview',
            path: pathNewFormat,
        }
        : false;
    // Finally, construct and return the category node
    const routes = indexRoute ? [indexRoute, ...nestedRoutes] : nestedRoutes;
    return {
        title: formatTitle(title),
        routes,
    };
}
function convertNavLeaf(navJsNode, collectedFrontmatter, pathStack, subfolder) {
    //  The path we want for our new format does NOT contain
    // the content subfolder or the file extension (always `.mdx`)
    const pathNewFormat = pathStack.concat(navJsNode).join('/');
    // The path we need to match from the older format
    // includes the content subfolder as well as the file extension
    const pathToMatch = path_1.default.join(subfolder, `${pathNewFormat}.mdx`);
    // We filter for matching frontmatter to get the "title" for the nav leaf.
    // We throw an error if there is no matching resource - there should be!
    const matchedFrontmatter = collectedFrontmatter.filter((resource) => {
        return resource.__resourcePath === pathToMatch;
    })[0];
    if (!matchedFrontmatter) {
        throw new Error(`Could not find frontmatter for resource path ${pathToMatch}.`);
    }
    // We pull the title from frontmatter.
    // We throw an error if there is no title in frontmatter - there should be!
    const { sidebar_title, page_title } = matchedFrontmatter;
    const title = sidebar_title || page_title;
    if (!title) {
        throw new Error(`Could not find title in frontmatter of ${pathToMatch}.`);
    }
    // Return the new format for the nav leaf
    return { title: formatTitle(title), path: pathNewFormat };
}
async function collectFrontmatter(inputDir, fileFilter) {
    // Traverse directories and parse frontmatter
    const targetFilepaths = klaw_sync_1.default(inputDir, {
        traverseAll: true,
        filter: fileFilter,
    }).map((f) => f.path);
    const collectedFrontmatter = await Promise.all(targetFilepaths.map(async (filePath) => {
        const rawFile = fs_1.default.readFileSync(filePath, 'utf-8');
        const { data: frontmatter } = gray_matter_1.default(rawFile);
        const __resourcePath = path_1.default.relative(inputDir, filePath);
        return { __resourcePath, ...frontmatter };
    }));
    return collectedFrontmatter;
}
function formatTitle(title) {
    return title.replace(/<tt>/g, '<code>').replace(/<\/tt>/g, '</code>');
}
