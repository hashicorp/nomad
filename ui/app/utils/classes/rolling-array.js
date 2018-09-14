// An array with a max length.
//
// When max length is surpassed, items are removed from
// the front of the array.

// Using Classes to extend Array is unsupported in Babel so this less
// ideal approach is taken: https://babeljs.io/docs/en/caveats#classes
export default function RollingArray(maxLength, ...items) {
  const array = new Array(...items);
  array.maxLength = maxLength;

  // Capture the originals of each array method, but
  // associate them with the array to prevent closures.
  array._push = array.push;
  array._splice = array.splice;
  array._unshift = array.unshift;

  // All mutable array methods build on top of insertAt
  array._insertAt = array.insertAt;

  // Bring the length back down to maxLength by removing from the front
  array._limit = function() {
    const surplus = this.length - this.maxLength;
    if (surplus > 0) {
      this.splice(0, surplus);
    }
  };

  array.push = function(...items) {
    this._push(...items);
    this._limit();
    return this.length;
  };

  array.splice = function(...args) {
    const returnValue = this._splice(...args);
    this._limit();
    return returnValue;
  };

  array.insertAt = function(...args) {
    const returnValue = this._insertAt(...args);
    this._limit();
    this.arrayContentDidChange();
    return returnValue;
  };

  array.unshift = function() {
    throw new Error('Cannot unshift onto a RollingArray');
  };

  return array;
}
