const invariant = (truthy, error) => {
  if (!truthy) throw new Error(error);
};

export function getCodeMirrorInstance(container) {
  return function(selector) {
    const cmService = container.lookup('service:code-mirror');

    const element = document.querySelector(selector);
    invariant(element, `Selector ${selector} matched no elements`);

    const cm = cmService.instanceFor(element.id);
    invariant(cm, `No registered CodeMirror instance for ${selector}`);

    return cm;
  };
}

export default function setupCodeMirror(hooks) {
  hooks.beforeEach(function() {
    this.getCodeMirrorInstance = getCodeMirrorInstance(this.owner);

    // Expose to window for access from page objects
    window.getCodeMirrorInstance = this.getCodeMirrorInstance;
  });

  hooks.afterEach(function() {
    delete window.getCodeMirrorInstance;
    delete this.getCodeMirrorInstance;
  });
}
