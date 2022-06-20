/**
 * Changes a JSON object into a string
 */

import { helper } from '@ember/component/helper';

export default helper(function stringifyObject(positional /*, named*/) {
  return JSON.stringify(positional[0], null, 2);
});
