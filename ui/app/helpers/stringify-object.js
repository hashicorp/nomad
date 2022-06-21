/**
 * Changes a JSON object into a string
 */

import { helper } from '@ember/component/helper';

function stringifyObject([obj], { replacer = null, whitespace = 2 }) {
  return JSON.stringify(obj, replacer, whitespace);
}

export default helper(stringifyObject);
