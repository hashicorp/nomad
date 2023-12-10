/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// An array with a max length.
//
// When max length is surpassed, items are removed from
// the front of the array.

// Native array methods
let { push, splice } = Array.prototype;

// Ember array prototype extension
let { insertAt } = Array.prototype;

// Using Classes to extend Array is unsupported in Babel so this less
// ideal approach is taken: https://babeljs.io/docs/en/caveats#classes
export default function RollingArray(maxLength, ...items) {
  const array = new Array(...items);
  array.maxLength = maxLength;

  // Bring the length back down to maxLength by removing from the front
  array._limit = function () {
    const surplus = this.length - this.maxLength;
    if (surplus > 0) {
      this.splice(0, surplus);
    }
  };

  array.push = function (...items) {
    push.apply(this, items);
    this._limit();
    return this.length;
  };

  array.splice = function (...args) {
    const returnValue = splice.apply(this, args);
    this._limit();
    return returnValue;
  };

  // All mutable array methods build on top of insertAt
  array.insertAt = function (...args) {
    const returnValue = insertAt.apply(this, args);
    this._limit();
    this.arrayContentDidChange();
    return returnValue;
  };

  array.unshift = function () {
    throw new Error('Cannot unshift onto a RollingArray');
  };

  return array;
}
