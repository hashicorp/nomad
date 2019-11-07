import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { selectChoose } from 'ember-power-select/test-support';
import { setupMirage } from 'ember-cli-mirage/test-support';
import JobsList from 'nomad-ui/tests/pages/jobs/list';
import ClientsList from 'nomad-ui/tests/pages/clients/list';
import PageLayout from 'nomad-ui/tests/pages/layout';
import Allocation from 'nomad-ui/tests/pages/allocations/detail';

module('Acceptance | regions (only one)', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function() {
    server.create('agent');
    server.create('node');
    server.createList('job', 2, { createAllocations: false, noDeployments: true });
  });

  test('when there is only one region, the region switcher is not shown in the nav bar and the region is not in the page title', async function(assert) {
    server.create('region', { id: 'global' });

    await JobsList.visit();

    assert.notOk(PageLayout.navbar.regionSwitcher.isPresent, 'No region switcher');
    assert.equal(document.title, 'Jobs - Nomad');
  });

  test('when the only region is not named "global", the region switcher still is not shown', async function(assert) {
    server.create('region', { id: 'some-region' });

    await JobsList.visit();

    assert.notOk(PageLayout.navbar.regionSwitcher.isPresent, 'No region switcher');
  });

  test('pages do not include the region query param', async function(assert) {
    server.create('region', { id: 'global' });

    await JobsList.visit();
    assert.equal(currentURL(), '/jobs', 'No region query param');

    const jobId = JobsList.jobs.objectAt(0).id;
    await JobsList.jobs.objectAt(0).clickRow();
    assert.equal(currentURL(), `/jobs/${jobId}`, 'No region query param');

    await ClientsList.visit();
    assert.equal(currentURL(), '/clients', 'No region query param');
  });

  test('api requests do not include the region query param', async function(assert) {
    server.create('region', { id: 'global' });

    await JobsList.visit();
    await JobsList.jobs.objectAt(0).clickRow();
    await PageLayout.gutter.visitClients();
    await PageLayout.gutter.visitServers();
    server.pretender.handledRequests.forEach(req => {
      assert.notOk(req.url.includes('region='), req.url);
    });
  });
});

module('Acceptance | regions (many)', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function() {
    server.create('agent');
    server.create('node');
    server.createList('job', 2, { createAllocations: false, noDeployments: true });
    server.create('allocation');
    server.create('region', { id: 'global' });
    server.create('region', { id: 'region-2' });
  });

  test('the region switcher is rendered in the nav bar and the region is in the page title', async function(assert) {
    await JobsList.visit();

    assert.ok(PageLayout.navbar.regionSwitcher.isPresent, 'Region switcher is shown');
    assert.equal(document.title, 'Jobs - global - Nomad');
  });

  test('when on the default region, pages do not include the region query param', async function(assert) {
    await JobsList.visit();

    assert.equal(currentURL(), '/jobs', 'No region query param');
    assert.equal(window.localStorage.nomadActiveRegion, 'global', 'Region in localStorage');
  });

  test('switching regions sets localStorage and the region query param', async function(assert) {
    const newRegion = server.db.regions[1].id;

    await JobsList.visit();

    await selectChoose('[data-test-region-switcher]', newRegion);

    assert.ok(
      currentURL().includes(`region=${newRegion}`),
      'New region is the region query param value'
    );
    assert.equal(window.localStorage.nomadActiveRegion, newRegion, 'New region in localStorage');
  });

  test('switching regions to the default region, unsets the region query param', async function(assert) {
    const startingRegion = server.db.regions[1].id;
    const defaultRegion = server.db.regions[0].id;

    await JobsList.visit({ region: startingRegion });

    await selectChoose('[data-test-region-switcher]', defaultRegion);

    assert.notOk(currentURL().includes('region='), 'No region query param for the default region');
    assert.equal(
      window.localStorage.nomadActiveRegion,
      defaultRegion,
      'New region in localStorage'
    );
  });

  test('switching regions on deep pages redirects to the application root', async function(assert) {
    const newRegion = server.db.regions[1].id;

    await Allocation.visit({ id: server.db.allocations[0].id });

    await selectChoose('[data-test-region-switcher]', newRegion);

    assert.ok(currentURL().includes('/jobs?'), 'Back at the jobs page');
  });

  test('navigating directly to a page with the region query param sets the application to that region', async function(assert) {
    const allocation = server.db.allocations[0];
    const region = server.db.regions[1].id;
    await Allocation.visit({ id: allocation.id, region });

    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}?region=${region}`,
      'Region param is persisted when navigating straight to a detail page'
    );
    assert.equal(
      window.localStorage.nomadActiveRegion,
      region,
      'Region is also set in localStorage from a detail page'
    );
  });

  test('when the region is not the default region, all api requests include the region query param', async function(assert) {
    const region = server.db.regions[1].id;

    await JobsList.visit({ region });

    await JobsList.jobs.objectAt(0).clickRow();
    await PageLayout.gutter.visitClients();
    await PageLayout.gutter.visitServers();
    const [regionsRequest, defaultRegionRequest, ...appRequests] = server.pretender.handledRequests;

    assert.notOk(
      regionsRequest.url.includes('region='),
      'The regions request is made without a region qp'
    );
    assert.notOk(
      defaultRegionRequest.url.includes('region='),
      'The default region request is made without a region qp'
    );

    appRequests.forEach(req => {
      assert.ok(req.url.includes(`region=${region}`), req.url);
    });
  });
});
