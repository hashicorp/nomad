/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';

function circularSafeReplacer(replacer) {
  const seen = new WeakSet();

  return function (key, value) {
    let nextValue = value;

    if (typeof replacer === 'function') {
      nextValue = replacer.call(this, key, value);
    }

    if (nextValue && typeof nextValue === 'object') {
      if (seen.has(nextValue)) {
        return '[Circular]';
      }
      seen.add(nextValue);
    }

    return nextValue;
  };
}

/**
 * Changes a JSON object into a string
 */
export function stringifyObject(
  [obj],
  { replacer = null, whitespace = 2 } = {}
) {
  try {
    return JSON.stringify(obj, replacer, whitespace);
  } catch (error) {
    if (error instanceof TypeError) {
      return JSON.stringify(obj, circularSafeReplacer(replacer), whitespace);
    }
    throw error;
  }
}

export default Helper.helper(stringifyObject);
