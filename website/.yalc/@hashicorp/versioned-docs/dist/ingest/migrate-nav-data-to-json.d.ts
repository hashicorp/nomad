export declare function loadJSNavData(pathToFile: string): Promise<any>;
export declare function getMigratedNavData(navJsData: $TSFixMe, contentDir: string): Promise<{
    navData: any;
    collectedFrontmatter: {
        __resourcePath: string;
    }[];
}>;
