import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Model | variable', function (hooks) {
  setupTest(hooks);

  test('it has basic fetchable properties', function (assert) {
    let store = this.owner.lookup('service:store');

    let model = store.createRecord('variable');
    model.setProperties({
      path: 'my/fun/path',
      namespace: 'default',
      keyValues: [
        { key: 'foo', value: 'bar' },
        { key: 'myVar', value: 'myValue' },
      ],
    });
    assert.ok(model.path);
    assert.equal(model.keyValues.length, 2);
  });

  test('it has a single keyValue by default', function (assert) {
    let store = this.owner.lookup('service:store');

    let model = store.createRecord('variable');
    model.setProperties({
      path: 'my/fun/path',
      namespace: 'default',
    });
    assert.equal(model.keyValues.length, 1);
  });
});
