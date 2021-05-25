import { VersionedDocument, VersionedNavData, VersionsMetadata } from './types';
export declare function writeDocument(document: VersionedDocument | VersionedNavData | VersionsMetadata): Promise<void>;
export declare function retrieveDocument(product: string, fullPath: string): Promise<VersionedDocument | VersionedNavData | VersionsMetadata | undefined>;
export declare function isVersionUploaded(product: string, sha: string): Promise<boolean | undefined>;
