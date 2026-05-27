/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { currentURL, settled } from '@ember/test-helpers';
import { getPageTitle } from 'ember-page-title/test-support';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { selectChoose } from 'ember-power-select/test-support';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import JobsList from 'nomad-ui/tests/pages/jobs/list';
import ClientsList from 'nomad-ui/tests/pages/clients/list';
import Layout from 'nomad-ui/tests/pages/layout';
import Allocation from 'nomad-ui/tests/pages/allocations/detail';
import Tokens from 'nomad-ui/tests/pages/settings/tokens';

module('Acceptance | regions (only one)', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    window.localStorage.clear();
    this.server.create('agent');
    this.server.create('node-pool');
    this.server.create('node');
    this.server.createList('job', 2, {
      createAllocations: false,
      noDeployments: true,
    });
  });

  test('it passes an accessibility audit', async function (assert) {
    await JobsList.visit();
    await a11yAudit(assert);
  });

  test('when there is only one region, and it is the default one, the region switcher is not shown in the nav bar and the region is not in the page title', async function (assert) {
    this.server.create('region', { id: 'global' });

    await JobsList.visit();

    assert.notOk(Layout.navbar.regionSwitcher.isPresent, 'No region switcher');
    assert.notOk(Layout.navbar.singleRegion.isPresent, 'No single region');
    assert.ok(getPageTitle().includes('Jobs'));
  });

  test('when the only region is not named "global", the region switcher still is not shown, but the single region name is', async function (assert) {
    this.server.create('region', { id: 'some-region' });

    await JobsList.visit();

    assert.notOk(Layout.navbar.regionSwitcher.isPresent, 'No region switcher');
    assert.ok(Layout.navbar.singleRegion.isPresent, 'Single region');
  });

  test('pages do not include the region query param', async function (assert) {
    this.server.create('region', { id: 'global' });

    await JobsList.visit();
    assert.deepEqual(currentURL(), '/jobs', 'No region query param');

    const jobId = JobsList.jobs.objectAt(0).id;
    await JobsList.jobs.objectAt(0).clickRow();
    assert.deepEqual(
      currentURL(),
      `/jobs/${jobId}@default`,
      'No region query param',
    );

    await ClientsList.visit();
    assert.deepEqual(currentURL(), '/clients', 'No region query param');
  });

  test('api requests do not include the region query param', async function (assert) {
    this.server.create('region', { id: 'global' });

    await JobsList.visit();
    await JobsList.jobs.objectAt(0).clickRow();
    await Layout.gutter.visitClients();
    await Layout.gutter.visitServers();
    this.server.pretender.handledRequests
      .filter((req) => !req.url.includes('/v1/status/leader'))
      .forEach((req) => {
        assert.notOk(req.url.includes('region='), req.url);
      });
  });
});

module('Acceptance | regions (many)', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    this.server.create('agent');
    this.server.create('node-pool');
    this.server.create('node');
    this.server.createList('job', 2, {
      createAllocations: false,
      noDeployments: true,
    });
    this.server.create('allocation');
    this.server.create('region', { id: 'global' });
    this.server.create('region', { id: 'region-2' });
  });

  test('the region switcher is rendered in the nav bar and the region is in the page title', async function (assert) {
    let managementToken = this.server.create('token');
    window.localStorage.nomadTokenSecret = managementToken.secretId;

    await JobsList.visit();
    await settled();

    assert.ok(
      Layout.navbar.regionSwitcher.isPresent,
      'Region switcher is shown',
    );
    assert.ok(getPageTitle().includes('Jobs - global'));
  });

  test('when on the default region, pages do not include the region query param', async function (assert) {
    let managementToken = this.server.create('token');
    window.localStorage.nomadTokenSecret = managementToken.secretId;
    await JobsList.visit();
    await settled();

    assert.deepEqual(currentURL(), '/jobs', 'No region query param');
    assert.deepEqual(
      window.localStorage.nomadActiveRegion,
      'global',
      'Region in localStorage',
    );
  });

  test('switching regions sets localStorage and the region query param', async function (assert) {
    const newRegion = this.server.db.regions[1].id;

    await JobsList.visit();

    await selectChoose('[data-test-region-switcher-parent]', newRegion);

    assert.ok(
      currentURL().includes(`region=${newRegion}`),
      'New region is the region query param value',
    );
    assert.deepEqual(
      window.localStorage.nomadActiveRegion,
      newRegion,
      'New region in localStorage',
    );
  });

  test('switching regions on a query-param-only transition refreshes the active route model', async function (assert) {
    const newRegion = this.server.db.regions[1].id;

    await JobsList.visit();

    const jobsRequestsBeforeSwitch =
      this.server.pretender.handledRequests.filter((request) =>
        request.url.includes('/v1/jobs'),
      ).length;

    await selectChoose('[data-test-region-switcher-parent]', newRegion);
    await settled();

    const jobsRequestsAfterSwitch =
      this.server.pretender.handledRequests.filter((request) =>
        request.url.includes('/v1/jobs'),
      );

    assert.ok(
      jobsRequestsAfterSwitch.length > jobsRequestsBeforeSwitch,
      'Jobs model request is issued again after region query-param switch'
    );

    assert.ok(
      jobsRequestsAfterSwitch
        .slice(jobsRequestsBeforeSwitch)
        .some((request) => request.url.includes(`region=${newRegion}`)),
      'Refreshed jobs request uses the selected region'
    );
  });

  test('switching regions on job detail reloads job, allocations, and evaluations', async function (assert) {
    const newRegion = this.server.db.regions[1].id;

    await JobsList.visit();
    const jobId = JobsList.jobs.objectAt(0).id;
    await JobsList.jobs.objectAt(0).clickRow();
    await settled();

    const isForJobPath = (request, path) => {
      const url = new URL(request.url, window.location.origin);
      return url.pathname === `/v1/job/${jobId}${path}`;
    };

    const jobRequestsBeforeSwitch =
      this.server.pretender.handledRequests.filter((request) =>
        isForJobPath(request, ''),
      ).length;

    const allocationRequestsBeforeSwitch =
      this.server.pretender.handledRequests.filter((request) =>
        isForJobPath(request, '/allocations'),
      ).length;

    const evaluationRequestsBeforeSwitch =
      this.server.pretender.handledRequests.filter((request) =>
        isForJobPath(request, '/evaluations'),
      ).length;

    await selectChoose('[data-test-region-switcher-parent]', newRegion);
    await settled();

    const jobRequestsAfterSwitch =
      this.server.pretender.handledRequests.filter((request) =>
        isForJobPath(request, ''),
      );
    const allocationRequestsAfterSwitch =
      this.server.pretender.handledRequests.filter((request) =>
        isForJobPath(request, '/allocations'),
      );
    const evaluationRequestsAfterSwitch =
      this.server.pretender.handledRequests.filter((request) =>
        isForJobPath(request, '/evaluations'),
      );

    assert.ok(
      jobRequestsAfterSwitch.length > jobRequestsBeforeSwitch,
      'Job record is fetched again after switching regions on job detail'
    );
    assert.ok(
      allocationRequestsAfterSwitch.length > allocationRequestsBeforeSwitch,
      'Job allocations are fetched again after switching regions on job detail'
    );
    assert.ok(
      evaluationRequestsAfterSwitch.length > evaluationRequestsBeforeSwitch,
      'Job evaluations are fetched again after switching regions on job detail'
    );

    assert.ok(
      jobRequestsAfterSwitch
        .slice(jobRequestsBeforeSwitch)
        .some((request) => request.url.includes(`region=${newRegion}`)),
      'Refetched job request includes selected region'
    );
    assert.ok(
      allocationRequestsAfterSwitch
        .slice(allocationRequestsBeforeSwitch)
        .some((request) => request.url.includes(`region=${newRegion}`)),
      'Refetched allocations request includes selected region'
    );
    assert.ok(
      evaluationRequestsAfterSwitch
        .slice(evaluationRequestsBeforeSwitch)
        .some((request) => request.url.includes(`region=${newRegion}`)),
      'Refetched evaluations request includes selected region'
    );
  });

  test('switching regions to the default region, unsets the region query param', async function (assert) {
    let managementToken = this.server.create('token');
    window.localStorage.nomadTokenSecret = managementToken.secretId;
    const startingRegion = this.server.db.regions[1].id;
    const defaultRegion = this.server.db.regions[0].id;

    await JobsList.visit({ region: startingRegion });
    await settled();
    await selectChoose('[data-test-region-switcher-parent]', defaultRegion);

    assert.notOk(
      currentURL().includes('region='),
      'No region query param for the default region',
    );
    assert.deepEqual(
      window.localStorage.nomadActiveRegion,
      defaultRegion,
      'New region in localStorage',
    );
  });

  test('navigating directly to a page with the region query param sets the application to that region', async function (assert) {
    const allocation = this.server.db.allocations[0];
    const region = this.server.db.regions[1].id;
    await Allocation.visit({ id: allocation.id, region });

    assert.deepEqual(
      currentURL(),
      `/allocations/${allocation.id}?region=${region}`,
      'Region param is persisted when navigating straight to a detail page',
    );
    assert.deepEqual(
      window.localStorage.nomadActiveRegion,
      region,
      'Region is also set in localStorage from a detail page',
    );
  });

  test('when the region is not the default region, all api requests other than the agent/self request include the region query param', async function (assert) {
    window.localStorage.removeItem('nomadTokenSecret');
    const region = this.server.db.regions[1].id;

    await JobsList.visit({ region });

    await JobsList.jobs.objectAt(0).clickRow();
    await Layout.gutter.visitClients();
    await Layout.gutter.visitServers();

    const regionsRequest = this.server.pretender.handledRequests.find((req) =>
      req.responseURL.includes('/v1/regions'),
    );
    const licenseRequest = this.server.pretender.handledRequests.find((req) =>
      req.responseURL.includes('/v1/operator/license'),
    );
    const appRequests = this.server.pretender.handledRequests.filter(
      (req) =>
        !req.responseURL.includes('/v1/regions') &&
        !req.responseURL.includes('/v1/operator/license') &&
        !req.responseURL.includes('/v1/agent/self') &&
        !req.responseURL.includes('/v1/agent/members') &&
        !req.responseURL.includes('/v1/acl/token/self') &&
        !req.responseURL.includes('/v1/acl/policy/anonymous') &&
        !req.responseURL.includes('/v1/search/fuzzy') &&
        !req.responseURL.includes('/v1/status/leader'),
    );

    assert.notOk(
      regionsRequest.url.includes('region='),
      'The regions request is made without a region qp',
    );
    assert.notOk(
      licenseRequest.url.includes('region='),
      'The default region request is made without a region qp',
    );

    appRequests.forEach((req) => {
      assert.ok(req.url.includes(`region=${region}`), req.url);
    });
  });

  test('Signing in sets the active region', async function (assert) {
    window.localStorage.clear();
    let managementToken = this.server.create('token');
    await Tokens.visit();
    assert.ok(
      ['Select a Region', 'Region: global'].includes(
        Layout.navbar.regionSwitcher.text,
      ),
      'Region picker shows either placeholder or default global region before signing in',
    );
    await Tokens.secret(managementToken.secretId).submit();
    assert.deepEqual(
      window.localStorage.nomadActiveRegion,
      'global',
      'Region is set in localStorage after signing in',
    );
    assert.deepEqual(
      Layout.navbar.regionSwitcher.text,
      'Region: global',
      'Region picker says "Region: global" after signing in',
    );
  });
});
