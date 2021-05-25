export declare const PRODUCT_DOMAIN_MAP: {
    readonly vault: "vaultproject.io";
    readonly terraform: "terraform.io";
    readonly consul: "consul.io";
    readonly vagrant: "vagrantup.com";
    readonly nomad: "nomadproject.io";
    readonly waypoint: "waypointproject.io";
    readonly cloud: "cloud.hashicorp.com";
    readonly packer: "packer.io";
    readonly boundary: "boundaryproject.io";
    readonly sentinel: "docs.hashicorp.com";
};
/**
 * Determines if the given URL is an internal URL within the context of the provided product
 * @param url URL to check
 * @param product associated product, if any
 * @returns
 */
export declare function isInternalUrl(url: string, product?: keyof typeof PRODUCT_DOMAIN_MAP): boolean;
/**
 * Inject a version segment into a path
 */
export declare function makeVersionedPath({ version, filePath, }: {
    version: string;
    filePath: string;
}): {
    fullPath: string;
    subpath: string;
};
/**
 * Extract the version from a path string, or an array of path segments
 */
export declare function getVersionFromPath(path?: string | string[]): string | undefined;
/**
 * Conditionally log a message
 *
 * @param condition Whether or not to log
 * @param msg The arguments passed to console.log to log
 */
export declare function logIf(condition: boolean, ...msg: string[]): void;
export declare function areRefsEqual(refA: string, refB: string): true | undefined;
