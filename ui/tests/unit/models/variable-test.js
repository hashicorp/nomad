import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Model | variable', function (hooks) {
  setupTest(hooks);

  test('it has basic fetchable properties', function (assert) {
    let store = this.owner.lookup('service:store');

    let model = store.createRecord('variable');
    model.setProperties({
      path: 'my/fun/path',
      namespace: 'toots',
      items: {
        foo: 'bar',
        myVar: 'myValue',
      },
    });
    assert.ok(model.path);
    assert.equal(Object.entries(model.items).length, 2);
  });
});
