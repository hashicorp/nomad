import { module, test } from 'qunit';
import { visit } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import PageLayout from 'nomad-ui/tests/pages/layout';
import { selectSearch } from 'ember-power-select/test-support';

module('Acceptance | search', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  test('search searches jobs', async function(assert) {
    server.create('node');
    server.create('job', { id: 'vwxyz', namespaceId: 'default' });
    server.create('job', { id: 'xyz', namespace: 'default' });
    server.create('job', { id: 'abc', namespace: 'default' });

    await visit('/');

    await selectSearch(PageLayout.navbar.search.scope, 'xy');

    PageLayout.navbar.search.as(search => {
      assert.equal(search.options.length, 2);
      assert.equal(search.options[0].text, 'xyz');
      assert.equal(search.options[1].text, 'vwxyz');
    });
  });

  test('clicking the search field starts search immediately', async function(assert) {
    await visit('/');

    assert.notOk(PageLayout.navbar.search.field.isPresent);

    await PageLayout.navbar.search.click();

    assert.ok(PageLayout.navbar.search.field.isPresent);
  });
});
