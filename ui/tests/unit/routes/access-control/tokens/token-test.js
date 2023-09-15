import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Route | access-control/tokens/token', function (hooks) {
  setupTest(hooks);

  test('it exists', function (assert) {
    let route = this.owner.lookup('route:access-control/tokens/token');
    assert.ok(route);
  });
});
