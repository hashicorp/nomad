/**
 * Checkout the provided git ref and execute the provided fn in the context of that ref
 */
export declare function checkoutRefAndExecute(ref: string, fn: () => Promise<void>): Promise<void>;
export declare function getGitSha(): Promise<any>;
