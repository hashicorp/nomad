// @ts-check
import Helper from '@ember/component/helper';

/**
 * Trims any number of slashes from the beginning and end of a string.
 * @param {Array<string>} params
 * @returns {string}
 */
export function trimPath([path = '']) {
  if (path.startsWith('/')) {
    path = trimPath([path.slice(1)]);
  }
  if (path.endsWith('/')) {
    path = trimPath([path.slice(0, -1)]);
  }
  return path;
}

export default Helper.helper(trimPath);
