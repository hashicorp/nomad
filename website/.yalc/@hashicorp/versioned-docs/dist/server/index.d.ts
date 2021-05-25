import { getVersionFromPath } from '../util';
import { VersionedDocument, VersionedNavData } from '../types';
export { makeServeStaticAssets } from './serve-static-asset';
/**
 * Returns a list of versions for use within the application, in the version selector for example.
 */
export declare function loadVersionList(product: string): Promise<{
    label: string;
    name: string;
}[]>;
export declare function loadVersionedNavData(product: string, basePath: string, version: string): Promise<VersionedNavData | undefined>;
export declare function loadVersionedDocument(product: string, fullPath: string): Promise<VersionedDocument | undefined>;
export { getVersionFromPath };
