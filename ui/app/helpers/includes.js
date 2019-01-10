import Helper from '@ember/component/helper';

/**
 * Includes
 *
 * Usage: {{includes needle haystack}}
 *
 * Returns true if an element (needle) is found in an array (haystack).
 */
export function includes([needle, haystack]) {
  return haystack.includes(needle);
}

export default Helper.helper(includes);
