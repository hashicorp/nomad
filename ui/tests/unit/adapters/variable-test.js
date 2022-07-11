import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Adapter | Variable', function (hooks) {
  setupTest(hooks);

  test('Correctly pluralizes lookups with shortened path', async function (assert) {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.adapterFor('variable');

    let newVariable = await this.store.createRecord('variable');

    assert.equal(
      this.subject().urlForFindAll('variable'),
      '/v1/vars',
      'pluralizes findAll lookup'
    );
    assert.equal(
      this.subject().urlForFindRecord('foo/bar', 'variable', newVariable),
      `/v1/var/${encodeURIComponent('foo/bar')}`,
      'singularizes findRecord lookup'
    );
  });
});
