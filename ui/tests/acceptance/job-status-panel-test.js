// @ts-check
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';

import {
  click,
  visit,
  find,
  findAll,
  fillIn,
  triggerEvent,
} from '@ember/test-helpers';

import { setupMirage } from 'ember-cli-mirage/test-support';
import faker from 'nomad-ui/mirage/faker';
import percySnapshot from '@percy/ember';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
// TODO: Mirage is not type-friendly / assigns "server" as a global. Try to work around this shortcoming.

module('Acceptance | job status panel', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    server.create('node');
  });

  test('Status panel lets you switch between Current and Historical', async function (assert) {
    assert.expect(5);
    let job = server.create('job', {
      status: 'running',
      datacenters: ['*'],
      type: 'service',
      createAllocations: true,
    });

    await visit(`/jobs/${job.id}`);
    assert.dom('.job-status-panel').exists();
    await a11yAudit(assert);
    await percySnapshot(assert, {
      percyCSS: `
        .allocation-row td { display: none; }
      `,
    });

    assert
      .dom('[data-test-status-mode="current"]')
      .exists('Current mode by default');

    await click('[data-test-status-mode-current]');

    assert
      .dom('[data-test-status-mode="current"]')
      .exists('Clicking active mode makes no change');

    await click('[data-test-status-mode-historical]');

    assert
      .dom('[data-test-status-mode="historical"]')
      .exists('Lets you switch to historical mode');
  });

  test('Status panel observes query parameters for current/historical', async function (assert) {
    assert.expect(2);
    let job = server.create('job', {
      status: 'running',
      datacenters: ['*'],
      type: 'service',
      createAllocations: true,
    });

    await visit(`/jobs/${job.id}?statusMode=historical`);
    assert.dom('.job-status-panel').exists();

    assert
      .dom('[data-test-status-mode="historical"]')
      .exists('Historical mode when rendered with queryParams');
  });

  test('Status Panel shows accurate number and types of ungrouped allocation blocks', async function (assert) {
    assert.expect(7);

    faker.seed(1);

    let groupTaskCount = 10;

    let job = server.create('job', {
      status: 'running',
      datacenters: ['*'],
      type: 'service',
      resourceSpec: ['M: 256, C: 500'], // a single group
      createAllocations: true,
      allocStatusDistribution: {
        running: 1,
        failed: 0,
        unknown: 0,
        lost: 0,
      },
      groupTaskCount,
      shallow: true,
    });

    await visit(`/jobs/${job.id}`);
    assert.dom('.job-status-panel').exists();

    let jobAllocCount = server.db.allocations.where({
      jobId: job.id,
    }).length;

    assert.equal(
      jobAllocCount,
      groupTaskCount * job.taskGroups.length,
      'Correect number of allocs generated (metatest)'
    );
    assert
      .dom('.ungrouped-allocs .represented-allocation.running')
      .exists(
        { count: jobAllocCount },
        `All ${jobAllocCount} allocations are represented in the status panel`
      );

    groupTaskCount = 20;

    job = server.create('job', {
      status: 'running',
      datacenters: ['*'],
      type: 'service',
      resourceSpec: ['M: 256, C: 500'], // a single group
      createAllocations: true,
      allocStatusDistribution: {
        running: 0.5,
        failed: 0.5,
        unknown: 0,
        lost: 0,
      },
      groupTaskCount,
      noActiveDeployment: true,
      shallow: true,
    });

    await visit(`/jobs/${job.id}`);
    assert.dom('.job-status-panel').exists();

    let runningAllocCount = server.db.allocations.where({
      jobId: job.id,
      clientStatus: 'running',
    }).length;

    let failedAllocCount = server.db.allocations.where({
      jobId: job.id,
      clientStatus: 'failed',
    }).length;

    assert.equal(
      runningAllocCount + failedAllocCount,
      groupTaskCount * job.taskGroups.length,
      'Correect number of allocs generated (metatest)'
    );
    assert
      .dom('.ungrouped-allocs .represented-allocation.running')
      .exists(
        { count: runningAllocCount },
        `All ${runningAllocCount} running allocations are represented in the status panel`
      );
    assert
      .dom('.ungrouped-allocs .represented-allocation.failed')
      .exists(
        { count: failedAllocCount },
        `All ${failedAllocCount} failed allocations are represented in the status panel`
      );
    await percySnapshot(assert, {
      percyCSS: `
          .allocation-row td { display: none; }
        `,
    });
  });

  test('Status Panel groups allocations when they get past a threshold', async function (assert) {
    assert.expect(6);

    faker.seed(1);

    let groupTaskCount = 20;

    let job = server.create('job', {
      status: 'running',
      datacenters: ['*'],
      type: 'service',
      resourceSpec: ['M: 256, C: 500'], // a single group
      createAllocations: true,
      allocStatusDistribution: {
        running: 1,
        failed: 0,
        unknown: 0,
        lost: 0,
      },
      groupTaskCount,
      shallow: true,
    });

    await visit(`/jobs/${job.id}`);
    assert.dom('.job-status-panel').exists();

    let jobAllocCount = server.db.allocations.where({
      jobId: job.id,
    }).length;

    assert
      .dom('.ungrouped-allocs .represented-allocation.running')
      .exists(
        { count: jobAllocCount },
        `All ${jobAllocCount} allocations are represented in the status panel, ungrouped`
      );

    groupTaskCount = 40;

    job = server.create('job', {
      status: 'running',
      datacenters: ['*'],
      type: 'service',
      resourceSpec: ['M: 256, C: 500'], // a single group
      createAllocations: true,
      allocStatusDistribution: {
        running: 1,
        failed: 0,
        unknown: 0,
        lost: 0,
      },
      groupTaskCount,
      shallow: true,
    });

    await visit(`/jobs/${job.id}`);
    assert.dom('.job-status-panel').exists();

    jobAllocCount = server.db.allocations.where({
      jobId: job.id,
    }).length;

    // At standard test resolution, 40 allocations will attempt to display 20 ungrouped, and 20 grouped.
    let desiredUngroupedAllocCount = 20;
    assert
      .dom('.ungrouped-allocs .represented-allocation.running')
      .exists(
        { count: desiredUngroupedAllocCount },
        `${desiredUngroupedAllocCount} allocations are represented ungrouped`
      );

    assert
      .dom('.represented-allocation.rest')
      .exists('Allocations are numerous enough that a summary block exists');
    assert
      .dom('.represented-allocation.rest')
      .hasText(
        `+${groupTaskCount - desiredUngroupedAllocCount}`,
        'Summary block has the correct number of grouped allocs'
      );

    await percySnapshot(assert, {
      percyCSS: `
        .allocation-row td { display: none; }
      `,
    });
  });

  test('Status Panel groups allocations when they get past a threshold, multiple statuses', async function (assert) {
    let groupTaskCount = 50;

    let job = server.create('job', {
      status: 'running',
      datacenters: ['*'],
      type: 'service',
      resourceSpec: ['M: 256, C: 500'], // a single group
      createAllocations: true,
      allocStatusDistribution: {
        running: 0.5,
        failed: 0.3,
        pending: 0.1,
        lost: 0.1,
      },
      groupTaskCount,
      shallow: true,
    });

    await visit(`/jobs/${job.id}`);
    assert.dom('.job-status-panel').exists();

    // With 50 allocs split across 4 statuses distributed as above, we can expect 25 running, 16 failed, 6 pending, and 4 remaining.
    // At standard test resolution, each status will be ungrouped/grouped as follows:
    // 25 running: 9 ungrouped, 17 grouped
    // 15 failed: 5 ungrouped, 10 grouped
    // 5 pending: 0 ungrouped, 5 grouped
    // 5 lost: 0 ungrouped, 5 grouped. Represented as "Unplaced"

    assert
      .dom('.ungrouped-allocs .represented-allocation.running')
      .exists({ count: 9 }, '9 running allocations are represented ungrouped');
    assert
      .dom('.represented-allocation.rest.running')
      .exists(
        'Running allocations are numerous enough that a summary block exists'
      );
    assert
      .dom('.represented-allocation.rest.running')
      .hasText(
        '+16',
        'Summary block has the correct number of grouped running allocs'
      );

    assert
      .dom('.ungrouped-allocs .represented-allocation.failed')
      .exists({ count: 5 }, '5 failed allocations are represented ungrouped');
    assert
      .dom('.represented-allocation.rest.failed')
      .exists(
        'Failed allocations are numerous enough that a summary block exists'
      );
    assert
      .dom('.represented-allocation.rest.failed')
      .hasText(
        '+10',
        'Summary block has the correct number of grouped failed allocs'
      );

    assert
      .dom('.ungrouped-allocs .represented-allocation.pending')
      .exists({ count: 0 }, '0 pending allocations are represented ungrouped');
    assert
      .dom('.represented-allocation.rest.pending')
      .exists(
        'pending allocations are numerous enough that a summary block exists'
      );
    assert
      .dom('.represented-allocation.rest.pending')
      .hasText(
        '5',
        'Summary block has the correct number of grouped pending allocs'
      );

    assert
      .dom('.ungrouped-allocs .represented-allocation.unplaced')
      .exists({ count: 0 }, '0 unplaced allocations are represented ungrouped');
    assert
      .dom('.represented-allocation.rest.unplaced')
      .exists(
        'Unplaced allocations are numerous enough that a summary block exists'
      );
    assert
      .dom('.represented-allocation.rest.unplaced')
      .hasText(
        '5',
        'Summary block has the correct number of grouped unplaced allocs'
      );
    await percySnapshot(
      'Status Panel groups allocations when they get past a threshold, multiple statuses (full width)',
      {
        percyCSS: `
          .allocation-row td { display: none; }
        `,
      }
    );

    // Simulate a window resize event; will recompute how many of each ought to be grouped.

    // At 1100px, only running and failed allocations have some ungrouped allocs
    find('.page-body').style.width = '1100px';
    await triggerEvent(window, 'resize');

    await percySnapshot(
      'Status Panel groups allocations when they get past a threshold, multiple statuses (1100px)',
      {
        percyCSS: `
          .allocation-row td { display: none; }
        `,
      }
    );

    assert
      .dom('.ungrouped-allocs .represented-allocation.running')
      .exists({ count: 7 }, '7 running allocations are represented ungrouped');
    assert
      .dom('.represented-allocation.rest.running')
      .exists(
        'Running allocations are numerous enough that a summary block exists'
      );
    assert
      .dom('.represented-allocation.rest.running')
      .hasText(
        '+18',
        'Summary block has the correct number of grouped running allocs'
      );

    assert
      .dom('.ungrouped-allocs .represented-allocation.failed')
      .exists({ count: 4 }, '4 failed allocations are represented ungrouped');
    assert
      .dom('.represented-allocation.rest.failed')
      .exists(
        'Failed allocations are numerous enough that a summary block exists'
      );
    assert
      .dom('.represented-allocation.rest.failed')
      .hasText(
        '+11',
        'Summary block has the correct number of grouped failed allocs'
      );

    // At 500px, only running allocations have some ungrouped allocs. The rest are all fully grouped.
    find('.page-body').style.width = '800px';
    await triggerEvent(window, 'resize');

    await percySnapshot(
      'Status Panel groups allocations when they get past a threshold, multiple statuses (500px)',
      {
        percyCSS: `
          .allocation-row td { display: none; }
        `,
      }
    );

    assert
      .dom('.ungrouped-allocs .represented-allocation.running')
      .exists({ count: 4 }, '4 running allocations are represented ungrouped');
    assert
      .dom('.represented-allocation.rest.running')
      .exists(
        'Running allocations are numerous enough that a summary block exists'
      );
    assert
      .dom('.represented-allocation.rest.running')
      .hasText(
        '+21',
        'Summary block has the correct number of grouped running allocs'
      );

    assert
      .dom('.ungrouped-allocs .represented-allocation.failed')
      .doesNotExist('no failed allocations are represented ungrouped');
    assert
      .dom('.represented-allocation.rest.failed')
      .exists(
        'Failed allocations are numerous enough that a summary block exists'
      );
    assert
      .dom('.represented-allocation.rest.failed')
      .hasText(
        '15',
        'Summary block has the correct number of grouped failed allocs'
      );
  });

  module('deployment history', function () {
    test('Deployment history can be searched', async function (assert) {
      faker.seed(1);

      let groupTaskCount = 10;

      let job = server.create('job', {
        status: 'running',
        datacenters: ['*'],
        type: 'service',
        resourceSpec: ['M: 256, C: 500'], // a single group
        createAllocations: true,
        allocStatusDistribution: {
          running: 1,
          failed: 0,
          unknown: 0,
          lost: 0,
        },
        groupTaskCount,
        shallow: true,
        activeDeployment: true,
        version: 0,
      });

      let state = server.create('task-state');
      state.events = server.schema.taskEvents.where({ taskStateId: state.id });

      server.schema.allocations.where({ jobId: job.id }).update({
        taskStateIds: [state.id],
        jobVersion: 0,
      });

      await visit(`/jobs/${job.id}`);
      assert.dom('.job-status-panel').exists();

      const serverEvents = server.schema.taskEvents.where({
        taskStateId: state.id,
      });
      const shownEvents = findAll('.timeline-object');
      const jobAllocations = server.db.allocations.where({ jobId: job.id });
      assert.equal(
        shownEvents.length,
        serverEvents.length * jobAllocations.length,
        'All events are shown'
      );

      await fillIn(
        '[data-test-history-search] input',
        serverEvents.models[0].message
      );
      assert.equal(
        findAll('.timeline-object').length,
        jobAllocations.length,
        'Only events matching the search are shown'
      );

      await fillIn('[data-test-history-search] input', 'foo bar baz');
      assert
        .dom('[data-test-history-search-no-match]')
        .exists('No match message is shown');
    });
  });
});
