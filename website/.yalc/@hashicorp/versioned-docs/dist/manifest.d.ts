export interface Version {
    slug: string;
    display: string;
    ref: string;
    sha?: string;
    navData?: Record<string, string>;
}
declare type VersionManifest = Version[];
export declare function loadVersionManifest(cwd?: string): Promise<VersionManifest>;
export declare function writeVersionManifest(manifest: VersionManifest, cwd?: string): Promise<void>;
export {};
