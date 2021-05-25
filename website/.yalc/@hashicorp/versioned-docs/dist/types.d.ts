/**
 * Structure of a stored document, without the DynamoDB field boilerplate
 */
export interface VersionedDocument {
    created_at?: string;
    sha: string;
    version: string;
    product: string;
    subpath: string;
    fullPath: string;
    metadata?: Record<string, any>;
    markdownSource: string;
    mdxTransforms: string[];
}
export declare type NavNode = NavLeaf | NavDirectLink | NavDivider | NavBranch;
export interface NavLeaf {
    title: string;
    path: string;
}
export interface NavDirectLink {
    title: string;
    href: string;
}
export interface NavDivider {
    divider: true;
}
export interface NavBranch {
    title: string;
    routes: NavNode[];
}
/**
 * Structure of stored navigation data, without the DynamoDB field boilerplate
 */
export interface VersionedNavData {
    created_at?: string;
    sha: string;
    version: string;
    product: string;
    subpath: string;
    fullPath: string;
    navData: NavNode[];
}
export interface VersionMetadata {
    sha: string;
    ref: string;
    display: string;
    slug: string;
    directory: string;
    navData?: Record<string, string>;
    mdxTransforms?: string[];
    navDataTransforms?: string[];
    isLatest?: boolean;
}
export interface VersionsMetadata {
    product: string;
    fullPath: string;
    versions: VersionMetadata[];
}
