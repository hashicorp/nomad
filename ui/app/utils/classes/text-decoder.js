/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// This is a very incomplete polyfill for TextDecoder used only
// by browsers that don't provide one but still provide a ReadableStream
// interface for fetch.

// A complete polyfill exists if this becomes problematic:
// https://github.com/inexorabletash/text-encoding
export default window.TextDecoder ||
  function () {
    this.decode = function (value) {
      let text = '';
      for (let i = 3; i < value.byteLength; i++) {
        text += String.fromCharCode(value[i]);
      }
      return text;
    };
  };
