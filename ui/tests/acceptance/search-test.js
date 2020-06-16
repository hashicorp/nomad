import { module, test } from 'qunit';
import { visit } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import PageLayout from 'nomad-ui/tests/pages/layout';
import { selectSearch } from 'ember-power-select/test-support';

module('Acceptance | search', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  test('search searches jobs and nodes', async function(assert) {
    server.create('node', { name: 'xyz' });
    server.create('node', { name: 'aaa' });

    server.create('job', { id: 'vwxyz', namespaceId: 'default' });
    server.create('job', { id: 'xyz', namespace: 'default' });
    server.create('job', { id: 'abc', namespace: 'default' });

    await visit('/');

    await selectSearch(PageLayout.navbar.search.scope, 'xy');

    PageLayout.navbar.search.as(search => {
      assert.equal(search.groups.length, 2);

      search.groups[0].as(jobs => {
        assert.equal(jobs.name, 'Jobs (2)');
        assert.equal(jobs.options.length, 2);
        assert.equal(jobs.options[0].text, 'xyz');
        assert.equal(jobs.options[1].text, 'vwxyz');
      });

      search.groups[1].as(clients => {
        assert.equal(clients.name, 'Clients (1)');
        assert.equal(clients.options.length, 1);
        assert.equal(clients.options[0].text, 'xyz');
      });
    });
  });

  test('clicking the search field starts search immediately', async function(assert) {
    await visit('/');

    assert.notOk(PageLayout.navbar.search.field.isPresent);

    await PageLayout.navbar.search.click();

    assert.ok(PageLayout.navbar.search.field.isPresent);
  });
});
