import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Ability | variable', function (hooks) {
  setupTest(hooks);

  test('it exists', function (assert) {
    const ability = this.owner.lookup('ability:variable');
    assert.ok(ability);
  });
});
