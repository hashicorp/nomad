import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Route | access-control/policies', function (hooks) {
  setupTest(hooks);

  test('it exists', function (assert) {
    let route = this.owner.lookup('route:access-control/policies');
    assert.ok(route);
  });
});
