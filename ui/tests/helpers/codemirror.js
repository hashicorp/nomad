/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export function getCodeMirrorInstance() {
  return function () {
    return document.querySelector('.hds-code-editor__editor').editor;
  };
}

export default function setupCodeMirror(hooks) {
  hooks.beforeEach(function () {
    this.getCodeMirrorInstance = getCodeMirrorInstance(this.owner);

    // Expose to window for access from page objects
    window.getCodeMirrorInstance = this.getCodeMirrorInstance;
  });

  hooks.afterEach(function () {
    delete window.getCodeMirrorInstance;
    delete this.getCodeMirrorInstance;
  });
}
