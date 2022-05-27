import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Controller | variables/variable/index', function (hooks) {
  setupTest(hooks);

  // TODO: Replace this with your real tests.
  test('it exists', function (assert) {
    let controller = this.owner.lookup('controller:variables/variable/index');
    assert.ok(controller);
  });
});
