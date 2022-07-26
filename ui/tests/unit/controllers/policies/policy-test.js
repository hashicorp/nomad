import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Controller | policies/policy', function (hooks) {
  setupTest(hooks);

  // TODO: Replace this with your real tests.
  test('it exists', function (assert) {
    let controller = this.owner.lookup('controller:policies/policy');
    assert.ok(controller);
  });
});
