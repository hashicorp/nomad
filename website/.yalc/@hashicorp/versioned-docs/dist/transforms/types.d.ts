import { VersionedDocument } from '../types';
import { Version } from '../manifest';
/**
 * Context passed into transforms to provide data about the running environment. This should make testing easier as well
 */
export interface TransformContext {
    rootDir: string;
    cwd: string;
    basePaths: string[];
    version: Version;
}
/**
 * Transform which takes in a VersionedDocument and returns VersionedDocument with any transforms applied
 */
export interface Transform {
    id: string;
    transformer(document: VersionedDocument, ctx: TransformContext): Promise<VersionedDocument>;
}
