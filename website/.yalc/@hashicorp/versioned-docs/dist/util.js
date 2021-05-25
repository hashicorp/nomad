"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.areRefsEqual = exports.logIf = exports.getVersionFromPath = exports.makeVersionedPath = exports.isInternalUrl = exports.PRODUCT_DOMAIN_MAP = void 0;
exports.PRODUCT_DOMAIN_MAP = {
    vault: 'vaultproject.io',
    terraform: 'terraform.io',
    consul: 'consul.io',
    vagrant: 'vagrantup.com',
    nomad: 'nomadproject.io',
    waypoint: 'waypointproject.io',
    cloud: 'cloud.hashicorp.com',
    packer: 'packer.io',
    boundary: 'boundaryproject.io',
    sentinel: 'docs.hashicorp.com',
};
/**
 * Determines if the given URL is an internal URL within the context of the provided product
 * @param url URL to check
 * @param product associated product, if any
 * @returns
 */
function isInternalUrl(url, product) {
    // relative paths are internal
    if (url.startsWith('/'))
        return true;
    // Check the domain name of the URL if it's not relative. If it matches the domain for the supplied product, then it's not internal
    const { hostname } = new URL(url);
    if (product && hostname.endsWith(exports.PRODUCT_DOMAIN_MAP[product]))
        return true;
    return false;
}
exports.isInternalUrl = isInternalUrl;
/**
 * Inject a version segment into a path
 */
function makeVersionedPath({ version, filePath, }) {
    const pathSegments = filePath.split('/');
    let subpath;
    if (pathSegments[0] === '') {
        pathSegments.splice(0, 1);
    }
    // remove the rootDir
    pathSegments.splice(0, 1);
    // extract the subpath after removing the root dir
    subpath = pathSegments[0];
    // add the version to the path
    pathSegments.splice(1, 0, version);
    const lastItem = pathSegments[pathSegments.length - 1];
    // remove trailing index.mdx if it exists
    if (lastItem === 'index.mdx') {
        pathSegments.splice(pathSegments.length - 1, 1);
    }
    else {
        pathSegments[pathSegments.length - 1] = lastItem.split('.')[0];
    }
    return { fullPath: pathSegments.join('/'), subpath };
}
exports.makeVersionedPath = makeVersionedPath;
/**
 * Extract the version from a path string, or an array of path segments
 */
function getVersionFromPath(path = []) {
    let pathSegments = path;
    if (!Array.isArray(pathSegments))
        pathSegments = pathSegments.split('/');
    const version = pathSegments.find((el) => /^v\d+\.\d/.test(el));
    return version;
}
exports.getVersionFromPath = getVersionFromPath;
/**
 * Conditionally log a message
 *
 * @param condition Whether or not to log
 * @param msg The arguments passed to console.log to log
 */
function logIf(condition, ...msg) {
    if (condition)
        console.log(...msg);
}
exports.logIf = logIf;
function areRefsEqual(refA, refB) {
    if (refA === refB)
        return true;
}
exports.areRefsEqual = areRefsEqual;
