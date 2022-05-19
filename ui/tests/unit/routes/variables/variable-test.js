import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Route | variables/variable', function (hooks) {
  setupTest(hooks);

  test('it exists', function (assert) {
    let route = this.owner.lookup('route:variables/variable');
    assert.ok(route);
  });
});
