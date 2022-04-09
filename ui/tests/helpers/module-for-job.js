/* eslint-disable qunit/require-expect */
/* eslint-disable qunit/no-conditional-assertions */
import {
  click,
  currentRouteName,
  currentURL,
  visit,
} from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import JobDetail from 'nomad-ui/tests/pages/jobs/detail';
import setPolicy from 'nomad-ui/tests/utils/set-policy';

// moduleFor is an old Ember-QUnit API that is deprected https://guides.emberjs.com/v1.10.0/testing/unit-test-helpers/
// this is a misnomer in our context, because we're not using this API, however, the linter does not understand this
// the linter warning will go away if we rename this factory function to generateJobDetailsTests
// eslint-disable-next-line ember/no-test-module-for
export default function moduleForJob(
  title,
  context,
  jobFactory,
  additionalTests
) {
  let job;

  module(title, function (hooks) {
    setupApplicationTest(hooks);
    setupMirage(hooks);
    hooks.before(function () {
      if (context !== 'allocations' && context !== 'children') {
        throw new Error(
          `Invalid context provided to moduleForJob, expected either "allocations" or "children", got ${context}`
        );
      }
    });

    hooks.beforeEach(async function () {
      server.create('node');
      job = jobFactory();
      if (!job.namespace || job.namespace === 'default') {
        await JobDetail.visit({ id: job.id });
      } else {
        await JobDetail.visit({ id: `${job.id}@${job.namespace}` });
      }

      const hasClientStatus = ['system', 'sysbatch'].includes(job.type);
      if (context === 'allocations' && hasClientStatus) {
        await click("[data-test-accordion-summary-chart='allocation-status']");
      }
    });

    test('visiting /jobs/:job_id', async function (assert) {
      const expectedURL = job.namespace
        ? `/jobs/${job.name}@${job.namespace}`
        : `/jobs/${job.name}`;

      assert.equal(decodeURIComponent(currentURL()), expectedURL);
      assert.equal(document.title, `Job ${job.name} - Nomad`);
    });

    test('the subnav links to overview', async function (assert) {
      await JobDetail.tabFor('overview').visit();

      const expectedURL = job.namespace
        ? `/jobs/${job.name}@${job.namespace}`
        : `/jobs/${job.name}`;

      assert.equal(decodeURIComponent(currentURL()), expectedURL);
    });

    test('the subnav links to definition', async function (assert) {
      await JobDetail.tabFor('definition').visit();

      const expectedURL = job.namespace
        ? `/jobs/${job.name}@${job.namespace}/definition`
        : `/jobs/${job.name}/definition`;

      assert.equal(decodeURIComponent(currentURL()), expectedURL);
    });

    test('the subnav links to versions', async function (assert) {
      await JobDetail.tabFor('versions').visit();

      const expectedURL = job.namespace
        ? `/jobs/${job.name}@${job.namespace}/versions`
        : `/jobs/${job.name}/versions`;

      assert.equal(decodeURIComponent(currentURL()), expectedURL);
    });

    test('the subnav links to evaluations', async function (assert) {
      await JobDetail.tabFor('evaluations').visit();

      const expectedURL = job.namespace
        ? `/jobs/${job.name}@${job.namespace}/evaluations`
        : `/jobs/${job.name}/evaluations`;

      assert.equal(decodeURIComponent(currentURL()), expectedURL);
    });

    test('the title buttons are dependent on job status', async function (assert) {
      if (job.status === 'dead') {
        assert.ok(JobDetail.start.isPresent);
        assert.notOk(JobDetail.stop.isPresent);
        assert.notOk(JobDetail.execButton.isPresent);
      } else {
        assert.notOk(JobDetail.start.isPresent);
        assert.ok(JobDetail.stop.isPresent);
        assert.ok(JobDetail.execButton.isPresent);
      }
    });

    if (context === 'allocations') {
      test('allocations for the job are shown in the overview', async function (assert) {
        assert.ok(
          JobDetail.allocationsSummary.isPresent,
          'Allocations are shown in the summary section'
        );
        assert.ok(
          JobDetail.childrenSummary.isHidden,
          'Children are not shown in the summary section'
        );
      });

      test('clicking in an allocation row navigates to that allocation', async function (assert) {
        const allocationRow = JobDetail.allocations[0];
        const allocationId = allocationRow.id;

        await allocationRow.visitRow();

        assert.equal(
          currentURL(),
          `/allocations/${allocationId}`,
          'Allocation row links to allocation detail'
        );
      });

      test('clicking legend item navigates to a pre-filtered allocations table', async function (assert) {
        const legendItem =
          JobDetail.allocationsSummary.legend.clickableItems[1];
        const status = legendItem.label;
        await legendItem.click();

        const encodedStatus = encodeURIComponent(JSON.stringify([status]));
        const expectedURL = new URL(
          urlWithNamespace(
            `/jobs/${job.name}@default/clients?status=${encodedStatus}`,
            job.namespace
          ),
          window.location
        );
        const gotURL = new URL(currentURL(), window.location);
        assert.deepEqual(gotURL.path, expectedURL.path);
        assert.deepEqual(gotURL.searchParams, expectedURL.searchParams);
      });

      test('clicking in a slice takes you to a pre-filtered allocations table', async function (assert) {
        const slice = JobDetail.allocationsSummary.slices[1];
        const status = slice.label;
        await slice.click();

        const encodedStatus = encodeURIComponent(JSON.stringify([status]));
        const expectedURL = new URL(
          urlWithNamespace(
            `/jobs/${encodeURIComponent(
              job.name
            )}/allocations?status=${encodedStatus}`,
            job.namespace
          ),
          window.location
        );
        const gotURL = new URL(currentURL(), window.location);
        assert.deepEqual(gotURL.pathname, expectedURL.pathname);

        // Sort and compare URL query params.
        gotURL.searchParams.sort();
        expectedURL.searchParams.sort();
        assert.equal(
          gotURL.searchParams.toString(),
          expectedURL.searchParams.toString()
        );
      });
    }

    if (context === 'children') {
      test('children for the job are shown in the overview', async function (assert) {
        assert.ok(
          JobDetail.childrenSummary.isPresent,
          'Children are shown in the summary section'
        );
        assert.ok(
          JobDetail.allocationsSummary.isHidden,
          'Allocations are not shown in the summary section'
        );
      });
    }

    for (var testName in additionalTests) {
      test(testName, async function (assert) {
        await additionalTests[testName].call(this, job, assert);
      });
    }
  });
}

// moduleFor is an old Ember-QUnit API that is deprected https://guides.emberjs.com/v1.10.0/testing/unit-test-helpers/
// this is a misnomer in our context, because we're not using this API, however, the linter does not understand this
// the linter warning will go away if we rename this factory function to generateJobClientStatusTests
// eslint-disable-next-line ember/no-test-module-for
export function moduleForJobWithClientStatus(
  title,
  jobFactory,
  additionalTests
) {
  let job;

  module(title, function (hooks) {
    setupApplicationTest(hooks);
    setupMirage(hooks);

    hooks.beforeEach(async function () {
      const clients = server.createList('node', 3, {
        datacenter: 'dc1',
        status: 'ready',
      });
      job = jobFactory();
      clients.forEach((c) => {
        server.create('allocation', { jobId: job.id, nodeId: c.id });
      });
    });

    module('with node:read permissions', function (hooks) {
      hooks.beforeEach(async function () {
        // Displaying the job status in client requires node:read permission.
        setPolicy({
          id: 'node-read',
          name: 'node-read',
          rulesJSON: {
            Node: {
              Policy: 'read',
            },
          },
        });

        await visitJobDetailPage(job);
      });

      test('the subnav links to clients', async function (assert) {
        await JobDetail.tabFor('clients').visit();

        const expectedURL = job.namespace
          ? `/jobs/${job.id}@${job.namespace}/clients`
          : `/jobs/${job.id}/clients`;

        assert.equal(currentURL(), expectedURL);
      });

      test('job status summary is shown in the overview', async function (assert) {
        assert.ok(
          JobDetail.jobClientStatusSummary.statusBar.isPresent,
          'Summary bar is displayed in the Job Status in Client summary section'
        );
      });

      test('clicking legend item navigates to a pre-filtered clients table', async function (assert) {
        const legendItem =
          JobDetail.jobClientStatusSummary.statusBar.legend.clickableItems[0];
        const status = legendItem.label;
        await legendItem.click();

        const encodedStatus = encodeURIComponent(JSON.stringify([status]));
        const expectedURL = new URL(
          urlWithNamespace(
            `/jobs/${job.name}/clients?status=${encodedStatus}`,
            job.namespace
          ),
          window.location
        );
        const gotURL = new URL(currentURL(), window.location);
        assert.deepEqual(gotURL.path, expectedURL.path);
        assert.deepEqual(gotURL.searchParams, expectedURL.searchParams);
      });

      test('clicking in a slice takes you to a pre-filtered clients table', async function (assert) {
        const slice = JobDetail.jobClientStatusSummary.statusBar.slices[0];
        const status = slice.label;
        await slice.click();

        const encodedStatus = encodeURIComponent(JSON.stringify([status]));

        const expectedURL = job.namespace
          ? `/jobs/${job.name}@${job.namespace}/clients?status=${encodedStatus}`
          : `/jobs/${job.name}/clients?status=${encodedStatus}`;

        assert.deepEqual(currentURL(), expectedURL, 'url is correct');
      });

      for (var testName in additionalTests) {
        test(testName, async function (assert) {
          await additionalTests[testName].call(this, job, assert);
        });
      }
    });

    module('without node:read permissions', function (hooks) {
      hooks.beforeEach(async function () {
        // Test blank Node policy to mock lack of permission.
        setPolicy({
          id: 'node',
          name: 'node',
          rulesJSON: {},
        });

        await visitJobDetailPage(job);
      });

      test('the page handles presentations concerns regarding the user not having node:read permissions', async function (assert) {
        assert
          .dom("[data-test-tab='clients']")
          .doesNotExist(
            'Job Detail Sub Navigation should not render Clients tab'
          );

        assert
          .dom('[data-test-nodes-not-authorized]')
          .exists('Renders Not Authorized message');
      });

      test('/jobs/job/clients route is protected with authorization logic', async function (assert) {
        await visit(`/jobs/${job.id}/clients`);

        assert.equal(
          currentRouteName(),
          'jobs.job.index',
          'The clients route cannot be visited unless you have node:read permissions'
        );
      });
    });
  });
}

function urlWithNamespace(url, namespace) {
  if (!namespace || namespace === 'default') {
    return url;
  }

  const parts = url.split('?');
  const params = new URLSearchParams(parts[1]);
  params.set('namespace', namespace);

  return `${parts[0]}?${params.toString()}`;
}

async function visitJobDetailPage({ id, namespace }) {
  if (!namespace || namespace === 'default') {
    await JobDetail.visit({ id });
  } else {
    await JobDetail.visit({ id: `${id}@${namespace}` });
  }
}
