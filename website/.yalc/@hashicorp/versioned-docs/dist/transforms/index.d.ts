import { VersionedDocument } from '../types';
import { Transform, TransformContext } from './types';
export declare const BASE_TRANSFORMS: string[];
export declare const LATEST_TRANSFORMS: string[];
/**
 * Applies the specified transform to the given document
 *
 * @param document Document to apply the transform against
 * @param transformArg Transform ID or Transform object
 * @param rootDir Directory where the content is being pulled from
 */
export declare function applyTransformToDocument(document: VersionedDocument, transformArg: Transform | string, ctx: Omit<TransformContext, 'cwd'>): Promise<VersionedDocument>;
