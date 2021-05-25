import { NavNode } from '../types';
import { Version } from '../manifest';
interface RawDocument {
    content: string;
    filePath: string;
}
/**
 * Extracts documents from the specified directory
 */
export declare function extractDocuments(directory: string): Promise<RawDocument[]>;
/**
 * Extract navigation data from the old js format, or the new json format
 *
 * TODO: we are reading all of the content files as a result of this for frontmatter.
 *       Optimize so we are only reading the files once throughout the entire ETL process.
 */
export declare function extractNavData(directory: string, version: Version): Promise<[string, NavNode[]][]>;
/**
 * Adds the version segment to nav data links
 */
export declare function addVersionToNavDataLinks(version: string, navData: NavNode[]): NavNode[];
/**
 * Ingest a complete version of a product's documentation, including all pages as well as navigation data
 */
export declare function ingestDocumentationVersion({ product, version, directory, sha, isLatestVersion, }: {
    product: string;
    version: Version;
    directory: string;
    sha: string;
    isLatestVersion?: boolean;
}): Promise<{
    mdxTransforms: string[];
    navDataTransforms: string[];
}>;
export {};
