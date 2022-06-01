import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Route | variables/path', function (hooks) {
  setupTest(hooks);

  test('it exists', function (assert) {
    let route = this.owner.lookup('route:variables/path');
    assert.ok(route);
  });
});
