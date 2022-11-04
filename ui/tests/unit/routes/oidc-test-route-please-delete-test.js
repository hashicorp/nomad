import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Route | oidc-test-route-please-delete', function (hooks) {
  setupTest(hooks);

  test('it exists', function (assert) {
    let route = this.owner.lookup('route:oidc-test-route-please-delete');
    assert.ok(route);
  });
});
