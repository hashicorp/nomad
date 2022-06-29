const invariant = (truthy, error) => {
  if (!truthy) throw new Error(error);
};

export function getCodeMirrorInstance(container) {
  return function (selector) {
    return document.querySelector('.CodeMirror').CodeMirror;
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
