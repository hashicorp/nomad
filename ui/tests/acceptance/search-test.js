import { module, skip, test } from 'qunit';
import { currentURL, triggerKeyEvent, visit } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import PageLayout from 'nomad-ui/tests/pages/layout';
import { selectSearch } from 'ember-power-select/test-support';

module('Acceptance | search', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  test('search searches and choosing an item navigates to it', async function(assert) {
    const node = server.create('node');
    server.create('job', { id: 'xyz', namespaceId: 'default' });

    await visit('/');

    await selectSearch(PageLayout.navbar.search.scope, 'xy');

    PageLayout.navbar.search.as(search => {
      assert.equal(search.groups.length, 1);

      search.groups[0].as(jobs => {
        assert.equal(jobs.name, 'jobs');
        assert.equal(jobs.options.length, 1);
        assert.equal(jobs.options[0].text, 'xyz');
      });
    });

    await PageLayout.navbar.search.groups[0].options[0].click();
    assert.equal(currentURL(), '/jobs/xyz');

    const allocation = server.db.allocations.firstObject;
    await selectSearch(PageLayout.navbar.search.scope, allocation.id.substr(0, 3));
    await PageLayout.navbar.search.groups[0].options[0].click();
    assert.equal(currentURL(), `/allocations/${allocation.id}`);

    await selectSearch(PageLayout.navbar.search.scope, node.id.substr(0, 3));
    await PageLayout.navbar.search.groups[0].options[0].click();
    assert.equal(currentURL(), `/clients/${node.id}`);
  });

  skip('pressing slash focuses the search', async function(assert) {
    await visit('/');

    assert.notOk(PageLayout.navbar.search.field.isPresent);

    await triggerKeyEvent('.navbar', 'keydown', 'Slash');

    assert.ok(PageLayout.navbar.search.field.isPresent);
  });
});
