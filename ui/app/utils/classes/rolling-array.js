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

  array.push = function(...items) {
    const returnValue = this._push(...items);

    const surplus = this.length - this.maxLength;
    if (surplus > 0) {
      this.splice(0, surplus);
    }

    return returnValue;
  };

  array.splice = function(...args) {
    const returnValue = this._splice(...args);

    const surplus = this.length - this.maxLength;
    if (surplus > 0) {
      this._splice(0, surplus);
    }

    return returnValue;
  };

  array.unshift = function() {
    throw new Error('Cannot unshift onto a RollingArray');
  };

  return array;
}
