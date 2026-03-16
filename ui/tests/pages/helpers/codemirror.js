/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

// Like fillable, but for the CodeMirror editor
//
// Usage: fillIn: codeFillable('[data-test-editor]')
//        Page.fillIn(code);
export function codeFillable(selector) {
  return {
    isDescriptor: true,

    get() {
      return function (code) {
        const cm = window.getCodeMirrorInstance(selector);
        cm.setValue(code);
        return this;
      };
    },
  };
}

// Like text, but for the CodeMirror editor
//
// Usage: content: code('[data-test-editor]')
//        Page.code(); // some = [ 'string', 'of', 'code' ]
export function code(selector) {
  return {
    isDescriptor: true,

    get() {
      const cm = window.getCodeMirrorInstance(selector);
      return cm.getValue();
    },
  };
}
