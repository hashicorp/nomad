import * as pathToRegexp from 'path-to-regexp';
import { Transform } from './types';
interface RedirectCompiled {
    source: pathToRegexp.MatchFunction;
    destination: string | pathToRegexp.PathFunction;
}
export declare const loadRedirects: (cwd: string, version: string) => RedirectCompiled[];
/**
 * Checks for a matching redirect with the given URL and, if found,
 * applies the matching redirect.
 *
 * @param url URL to check for redirects with and apply to
 * @param redirects List of redirects which will be tested against the URL
 * @returns The redirected URL
 */
export declare const checkAndApplyRedirect: (url: string, redirects: RedirectCompiled[]) => string | false;
/**
 * Loads the redirects defined in redirects.js or redirects.next.js and attempts to apply them to any
 * matching links in the document.
 */
export declare const transformRewriteInternalRedirects: Transform;
export {};
