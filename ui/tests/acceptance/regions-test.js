import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import JobsList from 'nomad-ui/tests/pages/jobs/list';
import ClientsList from 'nomad-ui/tests/pages/clients/list';
import PageLayout from 'nomad-ui/tests/pages/layout';
import Allocation from 'nomad-ui/tests/pages/allocations/detail';

moduleForAcceptance('Acceptance | regions (only one)', {
  beforeEach() {
    server.create('agent');
    server.create('node');
    server.createList('job', 5);
  },
});

test('when there is only one region, the region switcher is not shown in the nav bar', function(assert) {
  server.create('region', { id: 'global' });

  andThen(() => {
    JobsList.visit();
  });

  andThen(() => {
    assert.notOk(PageLayout.navbar.regionSwitcher.isPresent, 'No region switcher');
  });
});

test('when the only region is not named "global", the region switcher still is not shown', function(assert) {
  server.create('region', { id: 'some-region' });

  andThen(() => {
    JobsList.visit();
  });

  andThen(() => {
    assert.notOk(PageLayout.navbar.regionSwitcher.isPresent, 'No region switcher');
  });
});

test('pages do not include the region query param', function(assert) {
  let jobId;

  server.create('region', { id: 'global' });

  andThen(() => {
    JobsList.visit();
  });
  andThen(() => {
    assert.equal(currentURL(), '/jobs', 'No region query param');
  });
  andThen(() => {
    jobId = JobsList.jobs.objectAt(0).id;
    JobsList.jobs.objectAt(0).clickRow();
  });
  andThen(() => {
    assert.equal(currentURL(), `/jobs/${jobId}`, 'No region query param');
  });
  andThen(() => {
    ClientsList.visit();
  });
  andThen(() => {
    assert.equal(currentURL(), '/clients', 'No region query param');
  });
});

test('api requests do not include the region query param', function(assert) {
  server.create('region', { id: 'global' });

  andThen(() => {
    JobsList.visit();
  });
  andThen(() => {
    JobsList.jobs.objectAt(0).clickRow();
  });
  andThen(() => {
    PageLayout.gutter.visitClients();
  });
  andThen(() => {
    PageLayout.gutter.visitServers();
  });
  andThen(() => {
    server.pretender.handledRequests.forEach(req => {
      assert.notOk(req.url.includes('region='), req.url);
    });
  });
});

moduleForAcceptance('Acceptance | regions (many)', {
  beforeEach() {
    server.create('agent');
    server.create('node');
    server.createList('job', 5);
    server.create('region', { id: 'global' });
    server.create('region', { id: 'region-2' });
  },
});

test('the region switcher is rendered in the nav bar', function(assert) {
  JobsList.visit();

  andThen(() => {
    assert.ok(PageLayout.navbar.regionSwitcher.isPresent, 'Region switcher is shown');
  });
});

test('when on the default region, pages do not include the region query param', function(assert) {
  JobsList.visit();

  andThen(() => {
    assert.equal(currentURL(), '/jobs', 'No region query param');
    assert.equal(window.localStorage.nomadActiveRegion, 'global', 'Region in localStorage');
  });
});

test('switching regions sets localStorage and the region query param', function(assert) {
  const newRegion = server.db.regions[1].id;

  JobsList.visit();

  selectChoose('[data-test-region-switcher]', newRegion);

  andThen(() => {
    assert.ok(
      currentURL().includes(`region=${newRegion}`),
      'New region is the region query param value'
    );
    assert.equal(window.localStorage.nomadActiveRegion, newRegion, 'New region in localStorage');
  });
});

test('switching regions to the default region, unsets the region query param', function(assert) {
  const startingRegion = server.db.regions[1].id;
  const defaultRegion = server.db.regions[0].id;

  JobsList.visit({ region: startingRegion });

  selectChoose('[data-test-region-switcher]', defaultRegion);

  andThen(() => {
    assert.notOk(currentURL().includes('region='), 'No region query param for the default region');
    assert.equal(
      window.localStorage.nomadActiveRegion,
      defaultRegion,
      'New region in localStorage'
    );
  });
});

test('switching regions on deep pages redirects to the application root', function(assert) {
  const newRegion = server.db.regions[1].id;

  Allocation.visit({ id: server.db.allocations[0].id });

  selectChoose('[data-test-region-switcher]', newRegion);

  andThen(() => {
    assert.ok(currentURL().includes('/jobs?'), 'Back at the jobs page');
  });
});

test('navigating directly to a page with the region query param sets the application to that region', function(assert) {
  const allocation = server.db.allocations[0];
  const region = server.db.regions[1].id;
  Allocation.visit({ id: allocation.id, region });

  andThen(() => {
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
});

test('when the region is not the default region, all api requests include the region query param', function(assert) {
  const region = server.db.regions[1].id;

  JobsList.visit({ region });

  andThen(() => {
    JobsList.jobs.objectAt(0).clickRow();
  });
  andThen(() => {
    PageLayout.gutter.visitClients();
  });
  andThen(() => {
    PageLayout.gutter.visitServers();
  });
  andThen(() => {
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
