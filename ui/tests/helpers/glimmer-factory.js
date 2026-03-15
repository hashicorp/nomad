/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/no-commented-tests */
import { setOwner } from '@ember/owner';
// We comment test to show an example of how to use the factory function

/*
  Used in glimmer component unit tests. Glimmer components should typically
  be tested with integration tests, but occasionally individual methods or
  properties have logic that isn't coupled to rendering or the DOM and can
  be better tested in a unit fashion.

  Use like

  setupGlimmerComponentFactory(hooks, 'my-component')

  test('testing my component', function(assert) {
    const component = this.createComponent({ hello: 'world' });
    assert.equal(component.args.hello, 'world');
  });
*/

export default function setupGlimmerComponentFactory(hooks, componentKey) {
  hooks.beforeEach(function () {
    this.createComponent = glimmerComponentInstantiator(
      this.owner,
      componentKey
    );
  });

  hooks.afterEach(function () {
    delete this.createComponent;
  });
}

// Look up the component class and construct an instance for unit tests.
function glimmerComponentInstantiator(owner, componentKey) {
  return (args = {}) => {
    const componentClass = owner.factoryFor(`component:${componentKey}`).class;
    const component = new componentClass(owner, args);
    setOwner(component, owner);
    return component;
  };
}
