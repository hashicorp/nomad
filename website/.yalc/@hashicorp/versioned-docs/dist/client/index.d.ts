/// <reference types="react" />
import { getVersionFromPath } from '../util';
export { getVersionFromPath };
export declare function normalizeVersion(version: string): string;
/**
 * Remove the version segment from a path string, or an array of path segments
 */
export declare function removeVersionFromPath(path?: string | string[]): string;
/**
 * Component which accepts a list of versions and renders a select component. Navigates to the new version on select
 */
export declare function VersionSelect({ versions, }: {
    versions: {
        label: string;
        name: string;
    }[];
}): JSX.Element;
