import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Route | policies/policy', function (hooks) {
  setupTest(hooks);

  test('it exists', function (assert) {
    let route = this.owner.lookup('route:policies/policy');
    assert.ok(route);
  });
});
