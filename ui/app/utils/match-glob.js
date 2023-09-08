/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

const WILDCARD_GLOB = '*';

/**
 *
 * @param {string} pattern
 * @param {string} comparable
 * @example matchGlob('foo', 'foo') // true
 * @example matchGlob('foo', 'bar') // false
 * @example matchGlob('foo*', 'footbar') // true
 * @example matchGlob('foo*', 'bar') // false
 * @example matchGlob('*foo', 'foobar') // false
 * @example matchGlob('*foo', 'barfoo') // true
 * @example matchGlob('foo*bar', 'footbar') // true
 * @returns {boolean}
 */
export default function matchGlob(pattern, comparable) {
  const parts = pattern?.split(WILDCARD_GLOB);
  const hasLeadingGlob = pattern?.startsWith(WILDCARD_GLOB);
  const hasTrailingGlob = pattern?.endsWith(WILDCARD_GLOB);
  const lastPartOfPattern = parts[parts.length - 1];
  const isPatternWithoutGlob = parts.length === 1 && !hasLeadingGlob;

  if (!pattern || !comparable || isPatternWithoutGlob) {
    return pattern === comparable;
  }

  if (pattern === WILDCARD_GLOB) {
    return true;
  }

  let subStringToMatchOn = comparable;
  for (let i = 0; i < parts.length; i++) {
    const part = parts[i];
    const idx = subStringToMatchOn?.indexOf(part);
    const doesStringIncludeSubPattern = idx > -1;
    const doesMatchOnFirstSubpattern = idx === 0;

    if (i === 0 && !hasLeadingGlob && !doesMatchOnFirstSubpattern) {
      return false;
    }

    if (!doesStringIncludeSubPattern) {
      return false;
    }

    subStringToMatchOn = subStringToMatchOn.slice(0, idx + comparable.length);
  }

  return hasTrailingGlob || comparable.endsWith(lastPartOfPattern);
}
