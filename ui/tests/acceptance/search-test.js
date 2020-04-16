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
    const job = server.create('job', { id: 'xyz', namespaceId: 'default' });

    await visit('/');

    await selectSearch(PageLayout.navbar.search.scope, 'xy');

    PageLayout.navbar.search.as(search => {
      assert.equal(search.groups.length, 1);

      search.groups[0].as(jobs => {
        assert.equal(jobs.name, 'Jobs (1)');
        assert.equal(jobs.options.length, 1);
        assert.equal(jobs.options[0].text, 'xyz');
        assert.equal(jobs.options[0].statusClass, job.status);
      });
    });

    await PageLayout.navbar.search.groups[0].options[0].click();
    assert.equal(currentURL(), '/jobs/xyz');

    const allocation = server.db.allocations.firstObject;
    await selectSearch(PageLayout.navbar.search.scope, allocation.id.substr(0, 3));

    assert.equal(PageLayout.navbar.search.groups[0].name, 'Allocations (1)');
    assert.equal(
      PageLayout.navbar.search.groups[0].options[0].statusClass,
      allocation.clientStatus
    );

    await PageLayout.navbar.search.groups[0].options[0].click();
    assert.equal(currentURL(), `/allocations/${allocation.id}`);

    await selectSearch(PageLayout.navbar.search.scope, node.id.substr(0, 3));

    assert.equal(PageLayout.navbar.search.groups[0].name, 'Clients (1)');
    assert.equal(PageLayout.navbar.search.groups[0].options[0].statusClass, node.status);

    await PageLayout.navbar.search.groups[0].options[0].click();
    assert.equal(currentURL(), `/clients/${node.id}`);
  });

  test('only allocation, client, and job search results show', async function(assert) {
    server.create('node');
    server.create('job', { id: 'xyz', namespaceId: 'default' });

    await visit('/');

    const evaluation = server.db.evaluations.firstObject;
    await selectSearch(PageLayout.navbar.search.scope, evaluation.id.substr(0, 3));

    assert.ok(PageLayout.navbar.search.hasNoMatches);
  });

  test('clicking the search field starts search immediately', async function(assert) {
    await visit('/');

    assert.notOk(PageLayout.navbar.search.field.isPresent);

    await PageLayout.navbar.search.click();

    assert.ok(PageLayout.navbar.search.field.isPresent);
  });

  skip('pressing slash focuses the search', async function(assert) {
    await visit('/');

    assert.notOk(PageLayout.navbar.search.field.isPresent);

    window.pl = PageLayout;
    await triggerKeyEvent('.global-search', 'keydown', 'Slash');
    await this.pauseTest();

    assert.ok(PageLayout.navbar.search.field.isPresent);
  });
});
