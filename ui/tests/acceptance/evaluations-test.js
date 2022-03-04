import { click, currentRouteName, visit } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import { Response } from 'ember-cli-mirage';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import {
  selectChoose,
  clickTrigger,
} from 'ember-power-select/test-support/helpers';

const getStandardRes = () => [
  {
    CreateIndex: 1249,
    CreateTime: 1640181894162724000,
    DeploymentID: '12efbb28-840e-7794-b215-a7b112e40a4f',
    ID: '5fb1b8cd-00f8-fff8-de0c-197dc37f5053',
    JobID: 'cores-example',
    JobModifyIndex: 694,
    ModifyIndex: 1251,
    ModifyTime: 1640181894167194000,
    Namespace: 'ted-lasso',
    Priority: 50,
    QueuedAllocations: {
      lb: 0,
      webapp: 0,
    },
    SnapshotIndex: 1249,
    Status: 'complete',
    TriggeredBy: 'job-register',
    Type: 'service',
  },
  {
    CreateIndex: 1304,
    CreateTime: 1640183201719510000,
    DeploymentID: '878435bf-7265-62b1-7902-d45c44b23b79',
    ID: '66cb98a6-7740-d5ef-37e4-fa0f8b1de44b',
    JobID: 'cores-example',
    JobModifyIndex: 1304,
    ModifyIndex: 1306,
    ModifyTime: 1640183201721418000,
    Namespace: 'default',
    Priority: 50,
    QueuedAllocations: {
      webapp: 0,
      lb: 0,
    },
    SnapshotIndex: 1304,
    Status: 'complete',
    TriggeredBy: 'job-register',
    Type: 'service',
  },
  {
    CreateIndex: 1267,
    CreateTime: 1640182198255685000,
    DeploymentID: '12efbb28-840e-7794-b215-a7b112e40a4f',
    ID: '78009518-574d-eee6-919a-e83879175dd3',
    JobID: 'cores-example',
    JobModifyIndex: 1250,
    ModifyIndex: 1274,
    ModifyTime: 1640182228112823000,
    Namespace: 'ted-lasso',
    PreviousEval: '84f1082f-3e6e-034d-6df4-c6a321e7bd63',
    Priority: 50,
    QueuedAllocations: {
      lb: 0,
    },
    SnapshotIndex: 1272,
    Status: 'complete',
    TriggeredBy: 'alloc-failure',
    Type: 'service',
    WaitUntil: '2021-12-22T14:10:28.108136Z',
  },
  {
    CreateIndex: 1322,
    CreateTime: 1640183505760099000,
    DeploymentID: '878435bf-7265-62b1-7902-d45c44b23b79',
    ID: 'c184f72b-68a3-5180-afd6-af01860ad371',
    JobID: 'cores-example',
    JobModifyIndex: 1305,
    ModifyIndex: 1329,
    ModifyTime: 1640183535540881000,
    Namespace: 'default',
    PreviousEval: '9a917a93-7bc3-6991-ffc9-15919a38f04b',
    Priority: 50,
    QueuedAllocations: {
      lb: 0,
    },
    SnapshotIndex: 1326,
    Status: 'complete',
    TriggeredBy: 'alloc-failure',
    Type: 'service',
    WaitUntil: '2021-12-22T14:32:15.539556Z',
  },
];

module('Acceptance | evaluations list', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  test('it passes an accessibility audit', async function (assert) {
    assert.expect(2);

    await visit('/evaluations');

    assert.equal(
      currentRouteName(),
      'evaluations.index',
      'The default route in evaluations is evaluations index'
    );

    await a11yAudit(assert);
  });

  test('it renders an empty message if there are no evaluations rendered', async function (assert) {
    await visit('/evaluations');

    assert
      .dom('[data-test-empty-evaluations-list]')
      .exists('We display empty table message.');
    assert
      .dom('[data-test-no-eval]')
      .exists('We display a message saying there are no evaluations.');
  });

  test('it renders a list of evaluations', async function (assert) {
    assert.expect(3);

    server.get('/evaluations', function (_server, fakeRequest) {
      assert.deepEqual(
        fakeRequest.queryParams,
        {
          namespace: '*',
          per_page: '25',
          status: '',
          next_token: '',
        },
        'Forwards the correct query parameters on default query when route initially loads'
      );
      return getStandardRes();
    });

    await visit('/evaluations');

    assert
      .dom('[data-test-eval-table]')
      .exists('Evaluations table should render');
    assert
      .dom('[data-test-evaluation]')
      .exists({ count: 4 }, 'Should render the correct number of evaluations');
  });

  test('it should enable filtering by evaluation status', async function (assert) {
    assert.expect(2);

    server.get('/evaluations', getStandardRes);

    await visit('/evaluations');

    server.get('/evaluations', function (_server, fakeRequest) {
      assert.deepEqual(
        fakeRequest.queryParams,
        {
          namespace: '*',
          per_page: '25',
          status: 'pending',
          next_token: '',
        },
        'It makes another server request using the options selected by the user'
      );
      return [];
    });

    await clickTrigger('[data-test-evaluation-status-facet]');
    await selectChoose('[data-test-evaluation-status-facet]', 'Pending');

    assert
      .dom('[data-test-no-eval-match]')
      .exists('Renders a message saying no evaluations match filter status');
  });

  module('page size', function (hooks) {
    hooks.afterEach(function () {
      // PageSizeSelect and the Evaluations Controller are both using localStorage directly
      // Will come back and invert the dependency
      window.localStorage.clear();
    });

    test('it is possible to change page size', async function (assert) {
      assert.expect(1);

      server.get('/evaluations', getStandardRes);

      await visit('/evaluations');

      server.get('/evaluations', function (_server, fakeRequest) {
        assert.deepEqual(
          fakeRequest.queryParams,
          {
            namespace: '*',
            per_page: '50',
            status: '',
            next_token: '',
          },
          'It makes a request with the per_page set by the user'
        );
        return getStandardRes();
      });

      await clickTrigger('[data-test-per-page]');
      await selectChoose('[data-test-per-page]', 50);
    });
  });

  module('pagination', function () {
    test('it should enable pagination by using next tokens', async function (assert) {
      assert.expect(7);

      server.get('/evaluations', function () {
        return new Response(
          200,
          { 'x-nomad-nexttoken': 'next-token-1' },
          getStandardRes()
        );
      });

      await visit('/evaluations');

      server.get('/evaluations', function (_server, fakeRequest) {
        assert.deepEqual(
          fakeRequest.queryParams,
          {
            namespace: '*',
            per_page: '25',
            status: '',
            next_token: 'next-token-1',
          },
          'It makes another server request using the options selected by the user'
        );
        return new Response(
          200,
          { 'x-nomad-nexttoken': 'next-token-2' },
          getStandardRes()
        );
      });

      assert
        .dom('[data-test-eval-pagination-next]')
        .isEnabled(
          'If there is a next-token in the API response the next button should be enabled.'
        );
      await click('[data-test-eval-pagination-next]');

      server.get('/evaluations', function (_server, fakeRequest) {
        assert.deepEqual(
          fakeRequest.queryParams,
          {
            namespace: '*',
            per_page: '25',
            status: '',
            next_token: 'next-token-2',
          },
          'It makes another server request using the options selected by the user'
        );
        return getStandardRes();
      });
      await click('[data-test-eval-pagination-next]');

      assert
        .dom('[data-test-eval-pagination-next]')
        .isDisabled('If there is no next-token, the next button is disabled.');

      assert
        .dom('[data-test-eval-pagination-prev]')
        .isEnabled(
          'After we transition to the next page, the previous page button is enabled.'
        );

      server.get('/evaluations', function (_server, fakeRequest) {
        assert.deepEqual(
          fakeRequest.queryParams,
          {
            namespace: '*',
            per_page: '25',
            status: '',
            next_token: 'next-token-1',
          },
          'It makes a request using the stored old token.'
        );
        return new Response(
          200,
          { 'x-nomad-nexttoken': 'next-token-2' },
          getStandardRes()
        );
      });

      await click('[data-test-eval-pagination-prev]');

      server.get('/evaluations', function (_server, fakeRequest) {
        assert.deepEqual(
          fakeRequest.queryParams,
          {
            namespace: '*',
            per_page: '25',
            status: '',
            next_token: '',
          },
          'When there are no more stored previous tokens, we will request with no next-token.'
        );
        return new Response(
          200,
          { 'x-nomad-nexttoken': 'next-token-1' },
          getStandardRes()
        );
      });

      await click('[data-test-eval-pagination-prev]');
    });

    test('it should clear all query parameters on refresh', async function (assert) {
      assert.expect(1);

      server.get('/evaluations', function () {
        return new Response(
          200,
          { 'x-nomad-nexttoken': 'next-token-1' },
          getStandardRes()
        );
      });

      await visit('/evaluations');

      server.get('/evaluations', function () {
        return getStandardRes();
      });

      await click('[data-test-eval-pagination-next]');

      await clickTrigger('[data-test-evaluation-status-facet]');
      await selectChoose('[data-test-evaluation-status-facet]', 'Pending');

      server.get('/evaluations', function (_server, fakeRequest) {
        assert.deepEqual(
          fakeRequest.queryParams,
          {
            namespace: '*',
            per_page: '25',
            status: '',
            next_token: '',
          },
          'It clears all query parameters when making a refresh'
        );
        return new Response(
          200,
          { 'x-nomad-nexttoken': 'next-token-1' },
          getStandardRes()
        );
      });

      await click('[data-test-eval-refresh]');
    });

    test('it should reset pagination when filters are applied', async function (assert) {
      assert.expect(1);

      server.get('/evaluations', function () {
        return new Response(
          200,
          { 'x-nomad-nexttoken': 'next-token-1' },
          getStandardRes()
        );
      });

      await visit('/evaluations');

      server.get('/evaluations', function () {
        return new Response(
          200,
          { 'x-nomad-nexttoken': 'next-token-2' },
          getStandardRes()
        );
      });

      await click('[data-test-eval-pagination-next]');

      server.get('/evaluations', getStandardRes);
      await click('[data-test-eval-pagination-next]');

      server.get('/evaluations', function (_server, fakeRequest) {
        assert.deepEqual(
          fakeRequest.queryParams,
          {
            namespace: '*',
            per_page: '25',
            status: 'pending',
            next_token: '',
          },
          'It clears all next token when filtered request is made'
        );
        return getStandardRes();
      });
      await clickTrigger('[data-test-evaluation-status-facet]');
      await selectChoose('[data-test-evaluation-status-facet]', 'Pending');
    });
  });

  module('resource linking', function () {
    test('it should generate a link to the job resource', async function (assert) {
      server.create('node');
      const job = server.create('job', { shallow: true });
      server.create('evaluation', { jobId: job.id });
      await visit('/evaluations');

      assert
        .dom('[data-test-evaluation-resource]')
        .hasText(
          job.name,
          'It conditionally renders the correct resource name'
        );
      await click('[data-test-evaluation-resource]');
      assert
        .dom('[data-test-job-name]')
        .includesText(job.name, 'We navigate to the correct job page.');
    });

    test('it should generate a link to the node resource', async function (assert) {
      const node = server.create('node');
      server.create('evaluation', { nodeId: node.id });
      await visit('/evaluations');

      const shortNodeId = node.id.split('-')[0];
      assert
        .dom('[data-test-evaluation-resource]')
        .hasText(
          shortNodeId,
          'It conditionally renders the correct resource name'
        );

      await click('[data-test-evaluation-resource]');

      assert
        .dom('[data-test-title]')
        .includesText(node.name, 'We navigate to the correct client page.');
    });
  });
});
