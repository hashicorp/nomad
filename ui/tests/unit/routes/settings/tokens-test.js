import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Route | settings/tokens', function (hooks) {
  setupTest(hooks);

  test('it exists', function (assert) {
    let route = this.owner.lookup('route:settings/tokens');
    assert.ok(route);
  });
});
