import { module, skip, test } from 'qunit';
import { currentURL, visit } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import PageLayout from 'nomad-ui/tests/pages/layout';
import { selectSearch } from 'ember-power-select/test-support';
import { keyDown } from 'ember-keyboard/test-support/test-helpers';

module('Acceptance | search', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  test('search searches jobs and nodes and navigates to chosen items', async function(assert) {
    server.create('node', { name: 'xyz' });
    const otherNode = server.create('node', { name: 'aaa' });

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

    await PageLayout.navbar.search.groups[0].options[0].click();
    assert.equal(currentURL(), '/jobs/xyz');

    await selectSearch(PageLayout.navbar.search.scope, otherNode.id.substr(0, 3));

    await PageLayout.navbar.search.groups[1].options[0].click();
    assert.equal(currentURL(), `/clients/${otherNode.id}`);
  });

  test('clicking the search field starts search immediately', async function(assert) {
    await visit('/');

    assert.notOk(PageLayout.navbar.search.field.isPresent);

    await PageLayout.navbar.search.click();

    assert.ok(PageLayout.navbar.search.field.isPresent);
  });

  // This DOES work but keyDown doesn’t seem quite right…
  skip('pressing slash starts a search', async function(assert) {
    await visit('/');

    assert.notOk(PageLayout.navbar.search.field.isPresent);
    await keyDown('/');
    assert.ok(PageLayout.navbar.search.field.isPresent);
  });
});
