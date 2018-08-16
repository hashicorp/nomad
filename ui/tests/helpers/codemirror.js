import { registerHelper } from '@ember/test';

const invariant = (truthy, error) => {
  if (!truthy) throw new Error(error);
};

export default function registerCodeMirrorHelpers() {
  registerHelper('getCodeMirrorInstance', function(app, selector) {
    const cmService = app.__container__.lookup('service:code-mirror');

    const element = document.querySelector(selector);
    invariant(element, `Selector ${selector} matched no elements`);

    const cm = cmService.instanceFor(element.id);
    invariant(cm, `No registered CodeMirror instance for ${selector}`);

    return cm;
  });
}
