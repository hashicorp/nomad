/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/require-expect */
import {
  currentURL,
  settled,
  click,
  triggerKeyEvent,
  typeIn,
  visit,
} from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import pageSizeSelect from './behaviors/page-size-select';
import JobsList from 'nomad-ui/tests/pages/jobs/list';
import percySnapshot from '@percy/ember';
import faker from 'nomad-ui/mirage/faker';

let managementToken, clientToken;

module('Acceptance | jobs list', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    // Required for placing allocations (a result of creating jobs)
    server.create('node-pool');
    server.create('node');

    managementToken = server.create('token');
    clientToken = server.create('token');

    window.localStorage.clear();
    window.localStorage.nomadTokenSecret = managementToken.secretId;
  });

  test('it passes an accessibility audit', async function (assert) {
    await JobsList.visit();
    await a11yAudit(assert);
  });

  test('visiting /jobs', async function (assert) {
    await JobsList.visit();

    assert.equal(currentURL(), '/jobs');
    assert.equal(document.title, 'Jobs - Nomad');
  });

  test('/jobs should list the first page of jobs sorted by modify index', async function (assert) {
    faker.seed(1);
    const jobsCount = JobsList.pageSize + 1;
    server.createList('job', jobsCount, { createAllocations: true });

    await JobsList.visit();

    await percySnapshot(assert);

    const sortedJobs = server.db.jobs
      .sortBy('id')
      .sortBy('modifyIndex')
      .reverse();
    assert.equal(JobsList.jobs.length, JobsList.pageSize);
    JobsList.jobs.forEach((job, index) => {
      assert.equal(job.name, sortedJobs[index].name, 'Jobs are ordered');
    });
  });

  test('each job row should contain information about the job', async function (assert) {
    server.createList('job', 2);
    const job = server.db.jobs.sortBy('modifyIndex').reverse()[0];

    await JobsList.visit();

    const store = this.owner.lookup('service:store');
    const jobInStore = await store.peekRecord(
      'job',
      `["${job.id}","${job.namespace}"]`
    );

    const jobRow = JobsList.jobs.objectAt(0);

    assert.equal(jobRow.name, job.name, 'Name');
    assert.notOk(jobRow.hasNamespace);
    assert.equal(jobRow.nodePool, job.nodePool, 'Node Pool');
    assert.equal(jobRow.link, `/ui/jobs/${job.id}@default`, 'Detail Link');
    assert.equal(
      jobRow.status,
      jobInStore.aggregateAllocStatus.label,
      'Status'
    );
    assert.equal(jobRow.type, typeForJob(job), 'Type');
  });

  test('each job row should link to the corresponding job', async function (assert) {
    server.create('job');
    const job = server.db.jobs[0];

    await JobsList.visit();
    await JobsList.jobs.objectAt(0).clickName();

    assert.equal(currentURL(), `/jobs/${job.id}@default`);
  });

  test('the new job button transitions to the new job page', async function (assert) {
    await JobsList.visit();
    await JobsList.runJobButton.click();

    assert.equal(currentURL(), '/jobs/run');
  });

  test('the job run button is disabled when the token lacks permission', async function (assert) {
    window.localStorage.nomadTokenSecret = clientToken.secretId;

    await JobsList.visit();

    assert.ok(JobsList.runJobButton.isDisabled);
  });

  test('the anonymous policy is fetched to check whether to show the job run button', async function (assert) {
    window.localStorage.removeItem('nomadTokenSecret');

    server.create('policy', {
      id: 'anonymous',
      name: 'anonymous',
      rulesJSON: {
        Namespaces: [
          {
            Name: 'default',
            Capabilities: ['list-jobs', 'submit-job'],
          },
        ],
      },
    });

    await JobsList.visit();
    assert.notOk(JobsList.runJobButton.isDisabled);
  });

  test('when there are no jobs, there is an empty message', async function (assert) {
    faker.seed(1);
    await JobsList.visit();

    await percySnapshot(assert);

    assert.ok(JobsList.isEmpty, 'There is an empty message');
    assert.equal(
      JobsList.emptyState.headline,
      'No Jobs',
      'The message is appropriate'
    );
  });

  test('when there are jobs, but no matches for a search result, there is an empty message', async function (assert) {
    server.create('job', { name: 'cat 1' });
    server.create('job', { name: 'cat 2' });

    await JobsList.visit();

    await JobsList.search.fillIn('dog');

    assert.ok(JobsList.isEmpty, 'The empty message is shown');
    assert.equal(
      JobsList.emptyState.headline,
      'No Matches',
      'The message is appropriate'
    );
  });

  test('searching resets the current page', async function (assert) {
    server.createList('job', JobsList.pageSize + 1, {
      createAllocations: false,
    });

    await JobsList.visit();
    await click('[data-test-pager="next"]');

    assert.ok(
      currentURL().includes('cursorAt'),
      'Page query param contains cursorAt'
    );

    await JobsList.search.fillIn('foobar');

    assert.equal(
      currentURL(),
      '/jobs?filter=Name%20contains%20%22foobar%22',
      'No page query param'
    );
  });

  test('when a cluster has namespaces, each job row includes the job namespace', async function (assert) {
    server.createList('namespace', 2);
    server.createList('job', 2);
    const job = server.db.jobs.sortBy('modifyIndex').reverse()[0];

    await JobsList.visit({ namespace: '*' });

    const jobRow = JobsList.jobs.objectAt(0);
    assert.equal(jobRow.namespace, job.namespaceId);
  });

  test('when the namespace query param is set, only matching jobs are shown', async function (assert) {
    server.createList('namespace', 2);
    const job1 = server.create('job', {
      namespaceId: server.db.namespaces[0].id,
    });
    const job2 = server.create('job', {
      namespaceId: server.db.namespaces[1].id,
    });

    await JobsList.visit();
    assert.equal(JobsList.jobs.length, 2, 'All jobs by default');

    const firstNamespace = server.db.namespaces[0];
    await JobsList.visit({ filter: `Namespace == ${firstNamespace.id}` });
    assert.equal(JobsList.jobs.length, 1, 'One job in the default namespace');
    assert.equal(
      JobsList.jobs.objectAt(0).name,
      job1.name,
      'The correct job is shown'
    );

    const secondNamespace = server.db.namespaces[1];
    await JobsList.visit({ filter: `Namespace == ${secondNamespace.id}` });

    assert.equal(
      JobsList.jobs.length,
      1,
      `One job in the ${secondNamespace.name} namespace`
    );
    assert.equal(
      JobsList.jobs.objectAt(0).name,
      job2.name,
      'The correct job is shown'
    );
  });

  test('when accessing jobs is forbidden, show a message with a link to the tokens page', async function (assert) {
    server.pretender.get('/v1/jobs/statuses', () => [403, {}, null]);

    await JobsList.visit();
    assert.equal(JobsList.error.title, 'Not Authorized');
    await percySnapshot(assert);

    await JobsList.error.seekHelp();
    assert.equal(currentURL(), '/settings/tokens');
  });

  test('when a gateway timeout error occurs, appropriate options are shown', async function (assert) {
    // Initial request is fine
    await JobsList.visit();

    assert.dom('#jobs-list-cache-warning').doesNotExist();

    server.pretender.get('/v1/jobs/statuses', () => [
      504,
      {
        errors: [
          {
            status: '504',
          },
        ],
      },
      null,
    ]);
    const controller = this.owner.lookup('controller:jobs.index');
    let currentParams = {
      per_page: 10,
    };

    await controller.watchJobIDs.perform(currentParams, 0);
    // Manually set its "isRunning" attribute for testing purposes
    // (existence of one of the buttons depends on blocking query running, which Ember testing doesnt really support)
    controller.watchJobIDs.isRunning = true;
    await settled();

    assert.dom('#jobs-list-cache-warning').exists();

    assert
      .dom('.flash-message.alert-critical')
      .exists('A toast error message pops up.');

    await percySnapshot(assert);

    await click('[data-test-pause-fetching]');
    assert
      .dom('.flash-message.alert-critical')
      .doesNotExist('Error message removed when fetrching is paused');
    assert.dom('#jobs-list-cache-warning').exists('Cache warning remains');

    server.pretender.get('/v1/jobs/statuses', () => [200, {}, null]);
    await click('[data-test-restart-fetching]');
    assert
      .dom('#jobs-list-cache-warning')
      .doesNotExist('Cache warning removed when fetching is restarted');
  });

  function typeForJob(job) {
    return job.periodic
      ? 'periodic'
      : job.parameterized
      ? 'parameterized'
      : job.type;
  }

  test('the jobs list page has appropriate faceted search options', async function (assert) {
    await JobsList.visit();

    assert.ok(
      JobsList.facets.namespace.isHidden,
      'Namespace facet not found (no namespaces)'
    );
    assert.ok(JobsList.facets.type.isPresent, 'Type facet found');
    assert.ok(JobsList.facets.status.isPresent, 'Status facet found');
    assert.ok(JobsList.facets.nodePool.isPresent, 'Node Pools facet found');
    assert.notOk(
      JobsList.facets.namespace.isPresent,
      'Namespace facet not found by default'
    );
  });

  testFacet('Namespace', {
    facet: JobsList.facets.namespace,
    paramName: 'namespace',
    expectedOptions: ['default', 'namespace-2'],
    dynamicStrings: true,
    async beforeEach() {
      server.create('namespace', { id: 'default' });
      server.create('namespace', { id: 'namespace-2' });
      server.createList('job', 2, { namespaceId: 'default' });
      server.createList('job', 2, { namespaceId: 'namespace-2' });
      await JobsList.visit();
    },
    filter(job, selection) {
      return selection.includes(job.namespaceId);
    },
  });

  testFacet('Type', {
    facet: JobsList.facets.type,
    paramName: 'type',
    expectedOptions: ['batch', 'service', 'system', 'sysbatch'],
    async beforeEach() {
      server.createList('job', 2, { createAllocations: false, type: 'batch' });
      server.createList('job', 2, {
        createAllocations: false,
        type: 'batch',
        periodic: true,
        childrenCount: 0,
      });
      server.createList('job', 2, {
        createAllocations: false,
        type: 'batch',
        parameterized: true,
        childrenCount: 0,
      });
      server.createList('job', 2, {
        createAllocations: false,
        type: 'service',
      });
      await JobsList.visit();
    },
    filter(job, selection) {
      let displayType = job.type;
      return selection.includes(displayType);
    },
  });

  testFacet('Status', {
    facet: JobsList.facets.status,
    paramName: 'status',
    expectedOptions: ['pending', 'running', 'dead'],
    async beforeEach() {
      server.createList('job', 2, {
        status: 'pending',
        createAllocations: false,
        childrenCount: 0,
      });
      server.createList('job', 2, {
        status: 'running',
        createAllocations: false,
        childrenCount: 0,
      });
      server.createList('job', 2, {
        status: 'dead',
        createAllocations: false,
        childrenCount: 0,
      });
      await JobsList.visit();
    },
    filter: (job, selection) => selection.includes(job.status),
  });

  test('when the facet selections result in no matches, the empty state states why', async function (assert) {
    server.createList('job', 2, {
      status: 'pending',
      createAllocations: false,
      childrenCount: 0,
    });

    await JobsList.visit();

    await JobsList.facets.status.toggle();
    await JobsList.facets.status.options.objectAt(1).toggle();
    assert.ok(JobsList.isEmpty, 'There is an empty message');
    assert.equal(
      JobsList.emptyState.headline,
      'No Matches',
      'The message is appropriate'
    );
  });

  test('the jobs list is immediately filtered based on query params', async function (assert) {
    server.create('job', { type: 'batch', createAllocations: false });
    server.create('job', { type: 'service', createAllocations: false });

    await JobsList.visit({ filter: 'Type == batch' });

    assert.equal(
      JobsList.jobs.length,
      1,
      'Only one job shown due to query param'
    );
  });

  test('when the user has a client token that has a namespace with a policy to run a job', async function (assert) {
    const READ_AND_WRITE_NAMESPACE = 'read-and-write-namespace';
    const READ_ONLY_NAMESPACE = 'read-only-namespace';

    server.create('namespace', { id: READ_AND_WRITE_NAMESPACE });
    server.create('namespace', { id: READ_ONLY_NAMESPACE });

    const policy = server.create('policy', {
      id: 'something',
      name: 'something',
      rulesJSON: {
        Namespaces: [
          {
            Name: READ_AND_WRITE_NAMESPACE,
            Capabilities: ['submit-job'],
          },
          {
            Name: READ_ONLY_NAMESPACE,
            Capabilities: ['list-job'],
          },
        ],
      },
    });

    clientToken.policyIds = [policy.id];
    clientToken.save();

    window.localStorage.nomadTokenSecret = clientToken.secretId;

    await JobsList.visit({ namespace: READ_AND_WRITE_NAMESPACE });
    assert.notOk(JobsList.runJobButton.isDisabled);

    await JobsList.visit({ namespace: READ_ONLY_NAMESPACE });
    assert.notOk(JobsList.runJobButton.isDisabled);
  });

  test('when the user has no client tokens that allow them to run a job', async function (assert) {
    const READ_AND_WRITE_NAMESPACE = 'read-and-write-namespace';
    const READ_ONLY_NAMESPACE = 'read-only-namespace';

    server.create('namespace', { id: READ_ONLY_NAMESPACE });

    const policy = server.create('policy', {
      id: 'something',
      name: 'something',
      rulesJSON: {
        Namespaces: [
          {
            Name: READ_ONLY_NAMESPACE,
            Capabilities: ['list-job'],
          },
        ],
      },
    });

    clientToken.policyIds = [policy.id];
    clientToken.save();

    window.localStorage.nomadTokenSecret = clientToken.secretId;

    await JobsList.visit({ namespace: READ_AND_WRITE_NAMESPACE });
    assert.ok(JobsList.runJobButton.isDisabled);

    await JobsList.visit({ namespace: READ_ONLY_NAMESPACE });
    assert.ok(JobsList.runJobButton.isDisabled);
  });

  pageSizeSelect({
    resourceName: 'job',
    pageObject: JobsList,
    pageObjectList: JobsList.jobs,
    async setup() {
      server.createList('job', JobsList.pageSize, {
        shallow: true,
        createAllocations: false,
      });
      await JobsList.visit();
    },
  });

  test('the run job button works when filters are set', async function (assert) {
    server.create('job', {
      name: 'un',
      createAllocations: false,
      childrenCount: 0,
      type: 'batch',
    });

    server.create('job', {
      name: 'deux',
      createAllocations: false,
      childrenCount: 0,
      type: 'system',
    });

    await JobsList.visit();

    await JobsList.facets.type.toggle();
    await JobsList.facets.type.options[0].toggle();

    await JobsList.runJobButton.click();
    assert.equal(currentURL(), '/jobs/run');
  });

  test('Parent/child jobs are displayed correctly', async function (assert) {
    localStorage.setItem('nomadPageSize', '10');
    createJobs(server, 5);

    let periodicJob = server.create('job', 'periodic', {
      name: 'periodic',
      id: 'periodic',
      childrenCount: 10,
    });

    // Set all children of that job to have a status of "running"
    server.db.jobs.where({ parentId: periodicJob.id }).forEach((job) => {
      server.db.jobs.update(job.id, { status: 'running' });
    });

    await JobsList.visit();

    assert.dom('[data-test-job-row="periodic"]').exists();
    assert
      .dom('.job-row')
      .exists(
        { count: 6 },
        'Even though a periodic job has 10 children, only the parent is shown'
      );

    assert.dom('.allocation-status-row').exists({ count: 5 });
    assert
      .dom('[data-test-job-row="periodic"] .allocation-status-row')
      .doesNotExist('Parent job doesnt have an allocs chart');

    assert
      .dom('[data-test-job-row="periodic"] [data-test-job-status]')
      .hasText('10 running jobs', 'Parent job status indicates running jobs');

    server.db.jobs.where({ parentId: periodicJob.id }).forEach((job) => {
      server.db.jobs.update(job.id, { status: 'dead' });
    });

    const controller = this.owner.lookup('controller:jobs.index');
    let currentParams = {
      per_page: 10,
    };

    // We have to wait for watchJobIDs to trigger the "dueling query" with watchJobs.
    // Since we can't await the watchJobs promise, we set a reasonably short timeout
    // to check the state of the list after the dueling query has completed.
    await controller.watchJobIDs.perform(currentParams, 0);

    let parentStatusUpdated = assert.async(); // watch for this to say "My tests oughta be passing by now"
    const duelingQueryUpdateTime = 200;

    assert.timeout(500);

    setTimeout(async () => {
      assert
        .dom('[data-test-job-row="periodic"] [data-test-job-status]')
        .hasText(
          '10 completed jobs',
          'Parent job status indicates complete jobs'
        );
      parentStatusUpdated();

      await click('[data-test-job-row="periodic"]');
      assert
        .dom('[data-test-child-job-row]')
        .exists({ count: 10 }, 'All children are shown');
    }, duelingQueryUpdateTime);

    await percySnapshot(assert);
    localStorage.removeItem('nomadPageSize');
  });

  test('aggregateAllocStatus reflects job status correctly', async function (assert) {
    const defaultJobParams = {
      createAllocations: true,
      shallow: true,
      resourceSpec: Array(1).fill('M: 257, C: 500'),
      groupAllocCount: 10,
      noActiveDeployment: true,
      noFailedPlacements: true,
      status: 'running',
      type: 'service',
    };

    server.create('job', {
      ...defaultJobParams,
      id: 'healthy-job',
      allocStatusDistribution: {
        running: 1,
      },
    });

    server.create('job', {
      ...defaultJobParams,
      id: 'degraded-job',
      allocStatusDistribution: {
        running: 0.9,
        failed: 0.1,
      },
    });

    server.create('job', {
      ...defaultJobParams,
      id: 'recovering-job',
      allocStatusDistribution: {
        running: 0.9,
        pending: 0.1,
      },
    });

    server.create('job', {
      ...defaultJobParams,
      id: 'completed-job',
      allocStatusDistribution: {
        complete: 1,
      },
      type: 'batch',
    });

    server.create('job', {
      ...defaultJobParams,
      id: 'running-job',
      allocStatusDistribution: {
        running: 1,
      },
      type: 'batch',
    });

    server.create('job', {
      ...defaultJobParams,
      id: 'failed-job',
      allocStatusDistribution: {
        failed: 1,
      },
    });

    server.create('job', {
      ...defaultJobParams,
      id: 'failed-garbage-collected-job',
      type: 'service',
      allocStatusDistribution: {
        unknown: 1,
      },
      status: 'running',
    });

    server.create('job', {
      ...defaultJobParams,
      id: 'stopped-job',
      type: 'service',
      allocStatusDistribution: {
        unknown: 1,
      },
      status: 'dead',
      stopped: true,
    });

    server.create('job', {
      ...defaultJobParams,
      id: 'deploying-job',
      allocStatusDistribution: {
        running: 0.5,
        pending: 0.5,
      },
      noActiveDeployment: false,
      activeDeployment: true,
    });

    server.create('job', {
      ...defaultJobParams,
      id: 'scaled-down-job',
      groupAllocCount: 0,
      status: 'dead',
    });

    server.create('job', {
      ...defaultJobParams,
      id: 'ancient-system-job',
      status: 'dead',
      type: 'system',
      groupAllocCount: 0,
    });

    await JobsList.visit();

    assert
      .dom('[data-test-job-row="healthy-job"] [data-test-job-status]')
      .hasText('Healthy', 'Healthy job is healthy');
    // and all the rest
    assert
      .dom('[data-test-job-row="degraded-job"] [data-test-job-status]')
      .hasText('Degraded', 'Degraded job is degraded');
    assert
      .dom('[data-test-job-row="recovering-job"] [data-test-job-status]')
      .hasText('Recovering', 'Recovering job is recovering');
    assert
      .dom('[data-test-job-row="completed-job"] [data-test-job-status]')
      .hasText('Complete', 'Completed job is completed');
    assert
      .dom('[data-test-job-row="running-job"] [data-test-job-status]')
      .hasText('Running', 'Running job is running');
    assert
      .dom('[data-test-job-row="failed-job"] [data-test-job-status]')
      .hasText('Failed', 'Failed job is failed');
    assert
      .dom(
        '[data-test-job-row="failed-garbage-collected-job"] [data-test-job-status]'
      )
      .hasText('Failed', 'Failed garbage collected job is failed');
    assert
      .dom('[data-test-job-row="stopped-job"] [data-test-job-status]')
      .hasText('Stopped', 'Stopped job is stopped');
    assert
      .dom('[data-test-job-row="deploying-job"] [data-test-job-status]')
      .hasText('Deploying', 'Deploying job is deploying');
    assert
      .dom('[data-test-job-row="scaled-down-job"] [data-test-job-status]')
      .hasText('Scaled Down', 'Scaled down job is scaled down');
    assert
      .dom('[data-test-job-row="ancient-system-job"] [data-test-job-status]')
      .hasText('Failed', 'System job with no allocs is failed');

    await percySnapshot(assert);
  });

  test('Jobs with schedule blocks indicate when a task is paused', async function (assert) {
    server.create('job', {
      name: 'regular-job-1',
      createAllocations: true,
    });
    server.create('job', {
      name: 'regular-job-2',
      createAllocations: true,
    });
    server.create('job', {
      name: 'time-based-job ',
      id: 'time-based-job',
      createAllocations: true,
      type: 'service',
      withPausedTasks: true,
      shallow: false,
      resourceSpec: Array(1).fill('M: 257, C: 500'),
      groupAllocCount: 1,
      groupTaskCount: 1,
      allocStatusDistribution: {
        running: 1,
      },
      noActiveDeployment: true,
      status: 'running',
      noFailedPlacements: true,
    });

    const allocID = server.db.allocations.findBy({
      jobId: 'time-based-job',
    }).id;
    const groupID = server.db.taskGroups.findBy({ jobId: 'time-based-job' }).id;
    const task = server.db.tasks.findBy({ taskGroupID: groupID });

    await JobsList.visit();

    assert.dom('[data-test-job-row="time-based-job"]').exists();
    assert
      .dom('[data-test-paused-task-indicator]')
      .exists({ count: 1 }, 'Paused task indicator is shown');
    await percySnapshot(assert);
    await click('[data-test-job-row="time-based-job"]');
    await click(`[data-test-allocation="${allocID}"]`);
    await click(`[data-test-task-row="${task.name}"]`);
    assert.dom('.time-based-alert').exists();
    await percySnapshot(assert);
  });

  module('Pagination', function () {
    module('Buttons are appropriately disabled', function () {
      test('when there are no jobs', async function (assert) {
        await JobsList.visit();
        assert.dom('[data-test-pager="first"]').doesNotExist();
        assert.dom('[data-test-pager="previous"]').doesNotExist();
        assert.dom('[data-test-pager="next"]').doesNotExist();
        assert.dom('[data-test-pager="last"]').doesNotExist();
        await percySnapshot(assert);
      });
      test('when there are fewer jobs than your page size setting', async function (assert) {
        localStorage.setItem('nomadPageSize', '10');
        createJobs(server, 5);
        await JobsList.visit();
        assert.dom('[data-test-pager="first"]').isDisabled();
        assert.dom('[data-test-pager="previous"]').isDisabled();
        assert.dom('[data-test-pager="next"]').isDisabled();
        assert.dom('[data-test-pager="last"]').isDisabled();
        await percySnapshot(assert);
        localStorage.removeItem('nomadPageSize');
      });
      test('when you have plenty of jobs', async function (assert) {
        localStorage.setItem('nomadPageSize', '10');
        createJobs(server, 25);
        await JobsList.visit();
        assert.dom('.job-row').exists({ count: 10 });
        assert.dom('[data-test-pager="first"]').isDisabled();
        assert.dom('[data-test-pager="previous"]').isDisabled();
        assert.dom('[data-test-pager="next"]').isNotDisabled();
        assert.dom('[data-test-pager="last"]').isNotDisabled();
        // Clicking next brings me to another full page
        await click('[data-test-pager="next"]');
        assert.dom('.job-row').exists({ count: 10 });
        assert.dom('[data-test-pager="first"]').isNotDisabled();
        assert.dom('[data-test-pager="previous"]').isNotDisabled();
        assert.dom('[data-test-pager="next"]').isNotDisabled();
        assert.dom('[data-test-pager="last"]').isNotDisabled();
        // clicking next again brings me to the last page, showing jobs 20-25
        await click('[data-test-pager="next"]');
        assert.dom('.job-row').exists({ count: 5 });
        assert.dom('[data-test-pager="first"]').isNotDisabled();
        assert.dom('[data-test-pager="previous"]').isNotDisabled();
        assert.dom('[data-test-pager="next"]').isDisabled();
        assert.dom('[data-test-pager="last"]').isDisabled();
        await percySnapshot(assert);
        localStorage.removeItem('nomadPageSize');
      });
    });
    module('Jobs are appropriately sorted by modify index', function () {
      test('on a single long page', async function (assert) {
        const jobsToCreate = 25;
        localStorage.setItem('nomadPageSize', '25');
        createJobs(server, jobsToCreate);
        await JobsList.visit();
        assert.dom('.job-row').exists({ count: 25 });
        // Check the data-test-modify-index attribute on each row
        let rows = document.querySelectorAll('.job-row');
        let modifyIndexes = Array.from(rows).map((row) =>
          parseInt(row.getAttribute('data-test-modify-index'))
        );
        assert.deepEqual(
          modifyIndexes,
          Array(jobsToCreate)
            .fill()
            .map((_, i) => i + 1)
            .reverse(),
          'Jobs are sorted by modify index'
        );
        localStorage.removeItem('nomadPageSize');
      });
      test('across multiple pages', async function (assert) {
        const jobsToCreate = 90;
        const pageSize = 25;
        localStorage.setItem('nomadPageSize', pageSize.toString());
        createJobs(server, jobsToCreate);
        await JobsList.visit();
        let rows = document.querySelectorAll('.job-row');
        let modifyIndexes = Array.from(rows).map((row) =>
          parseInt(row.getAttribute('data-test-modify-index'))
        );
        assert.deepEqual(
          modifyIndexes,
          Array(jobsToCreate)
            .fill()
            .map((_, i) => i + 1)
            .reverse()
            .slice(0, pageSize),
          'First page is sorted by modify index'
        );
        // Click next
        await click('[data-test-pager="next"]');
        rows = document.querySelectorAll('.job-row');
        modifyIndexes = Array.from(rows).map((row) =>
          parseInt(row.getAttribute('data-test-modify-index'))
        );
        assert.deepEqual(
          modifyIndexes,
          Array(jobsToCreate)
            .fill()
            .map((_, i) => i + 1)
            .reverse()
            .slice(pageSize, pageSize * 2),
          'Second page is sorted by modify index'
        );

        // Click next again
        await click('[data-test-pager="next"]');
        rows = document.querySelectorAll('.job-row');
        modifyIndexes = Array.from(rows).map((row) =>
          parseInt(row.getAttribute('data-test-modify-index'))
        );
        assert.deepEqual(
          modifyIndexes,
          Array(jobsToCreate)
            .fill()
            .map((_, i) => i + 1)
            .reverse()
            .slice(pageSize * 2, pageSize * 3),
          'Third page is sorted by modify index'
        );

        // Click previous
        await click('[data-test-pager="previous"]');
        rows = document.querySelectorAll('.job-row');
        modifyIndexes = Array.from(rows).map((row) =>
          parseInt(row.getAttribute('data-test-modify-index'))
        );
        assert.deepEqual(
          modifyIndexes,
          Array(jobsToCreate)
            .fill()
            .map((_, i) => i + 1)
            .reverse()
            .slice(pageSize, pageSize * 2),
          'Second page is sorted by modify index'
        );

        // Click next twice, should be the last page, and therefore fewer than pageSize jobs
        await click('[data-test-pager="next"]');
        await click('[data-test-pager="next"]');

        rows = document.querySelectorAll('.job-row');
        modifyIndexes = Array.from(rows).map((row) =>
          parseInt(row.getAttribute('data-test-modify-index'))
        );
        assert.deepEqual(
          modifyIndexes,
          Array(jobsToCreate)
            .fill()
            .map((_, i) => i + 1)
            .reverse()
            .slice(pageSize * 3),
          'Fourth page is sorted by modify index'
        );
        assert.equal(
          rows.length,
          jobsToCreate - pageSize * 3,
          'Last page has fewer jobs'
        );

        // Go back to the first page
        await click('[data-test-pager="first"]');
        rows = document.querySelectorAll('.job-row');
        modifyIndexes = Array.from(rows).map((row) =>
          parseInt(row.getAttribute('data-test-modify-index'))
        );
        assert.deepEqual(
          modifyIndexes,
          Array(jobsToCreate)
            .fill()
            .map((_, i) => i + 1)
            .reverse()
            .slice(0, pageSize),
          'First page is sorted by modify index'
        );

        // Click "last" to get an even number of jobs at the end of the list
        await click('[data-test-pager="last"]');
        rows = document.querySelectorAll('.job-row');
        modifyIndexes = Array.from(rows).map((row) =>
          parseInt(row.getAttribute('data-test-modify-index'))
        );
        assert.deepEqual(
          modifyIndexes,
          Array(jobsToCreate)
            .fill()
            .map((_, i) => i + 1)
            .reverse()
            .slice(-pageSize),
          'Last page is sorted by modify index'
        );
        assert.equal(
          rows.length,
          pageSize,
          'Last page has the correct number of jobs'
        );

        // type "{{" to go to the beginning
        triggerKeyEvent('.page-layout', 'keydown', '{');
        await triggerKeyEvent('.page-layout', 'keydown', '{');
        rows = document.querySelectorAll('.job-row');
        modifyIndexes = Array.from(rows).map((row) =>
          parseInt(row.getAttribute('data-test-modify-index'))
        );
        assert.deepEqual(
          modifyIndexes,
          Array(jobsToCreate)
            .fill()
            .map((_, i) => i + 1)
            .reverse()
            .slice(0, pageSize),
          'Keynav takes me back to the starting page'
        );

        // type "]]" to go forward a page
        triggerKeyEvent('.page-layout', 'keydown', ']');
        await triggerKeyEvent('.page-layout', 'keydown', ']');
        rows = document.querySelectorAll('.job-row');
        modifyIndexes = Array.from(rows).map((row) =>
          parseInt(row.getAttribute('data-test-modify-index'))
        );
        assert.deepEqual(
          modifyIndexes,
          Array(jobsToCreate)
            .fill()
            .map((_, i) => i + 1)
            .reverse()
            .slice(pageSize, pageSize * 2),
          'Keynav takes me forward a page'
        );

        localStorage.removeItem('nomadPageSize');
      });
    });
    module('Live updates are reflected in the list', function () {
      test('When you have live updates enabled, the list updates when new jobs are created', async function (assert) {
        localStorage.setItem('nomadPageSize', '10');
        createJobs(server, 10);
        await JobsList.visit();
        assert.dom('.job-row').exists({ count: 10 });
        let rows = document.querySelectorAll('.job-row');
        assert.equal(rows.length, 10, 'List is still 10 rows');
        let modifyIndexes = Array.from(rows).map((row) =>
          parseInt(row.getAttribute('data-test-modify-index'))
        );
        assert.deepEqual(
          modifyIndexes,
          Array(10)
            .fill()
            .map((_, i) => i + 1)
            .reverse(),
          'Jobs are sorted by modify index'
        );
        assert.dom('[data-test-pager="next"]').isDisabled();

        // Create a new job
        server.create('job', {
          namespaceId: 'default',
          resourceSpec: Array(1).fill('M: 256, C: 500'),
          groupAllocCount: 1,
          modifyIndex: 11,
          createAllocations: false,
          shallow: true,
          name: 'new-job',
        });

        const controller = this.owner.lookup('controller:jobs.index');

        let currentParams = {
          per_page: 10,
        };

        // We have to wait for watchJobIDs to trigger the "dueling query" with watchJobs.
        // Since we can't await the watchJobs promise, we set a reasonably short timeout
        // to check the state of the list after the dueling query has completed.
        await controller.watchJobIDs.perform(currentParams, 0);

        let updatedJob = assert.async(); // watch for this to say "My tests oughta be passing by now"
        const duelingQueryUpdateTime = 200;

        assert.timeout(500);

        setTimeout(async () => {
          // Order should now be 11-2
          rows = document.querySelectorAll('.job-row');
          modifyIndexes = Array.from(rows).map((row) =>
            parseInt(row.getAttribute('data-test-modify-index'))
          );
          assert.deepEqual(
            modifyIndexes,
            Array(10)
              .fill()
              .map((_, i) => i + 2)
              .reverse(),
            'Jobs are sorted by modify index'
          );

          // Simulate one of the on-page jobs getting its modify-index bumped. It should bump to the top of the list.
          let existingJobToUpdate = server.db.jobs.findBy(
            (job) => job.modifyIndex === 5
          );
          server.db.jobs.update(existingJobToUpdate.id, { modifyIndex: 12 });
          await controller.watchJobIDs.perform(currentParams, 0);
          let updatedOnPageJob = assert.async();

          setTimeout(async () => {
            rows = document.querySelectorAll('.job-row');
            modifyIndexes = Array.from(rows).map((row) =>
              parseInt(row.getAttribute('data-test-modify-index'))
            );
            assert.deepEqual(
              modifyIndexes,
              [12, 11, 10, 9, 8, 7, 6, 4, 3, 2],
              'Jobs are sorted by modify index, on-page job moves up to the top, and off-page pending'
            );
            updatedOnPageJob();

            assert.dom('[data-test-pager="next"]').isNotDisabled();

            await click('[data-test-pager="next"]');

            rows = document.querySelectorAll('.job-row');
            assert.equal(rows.length, 1, 'List is now 1 row');
            assert.equal(
              rows[0].getAttribute('data-test-modify-index'),
              '1',
              'Job is the first job, now pushed to the second page'
            );
          }, duelingQueryUpdateTime);
          updatedJob();
        }, duelingQueryUpdateTime);

        localStorage.removeItem('nomadPageSize');
      });
      test('When you have live updates disabled, the list does not update, but prompts you to refresh', async function (assert) {
        localStorage.setItem('nomadPageSize', '10');
        localStorage.setItem('nomadLiveUpdateJobsIndex', 'false');
        createJobs(server, 10);
        await JobsList.visit();
        assert.dom('[data-test-updates-pending-button]').doesNotExist();

        let rows = document.querySelectorAll('.job-row');
        assert.equal(rows.length, 10, 'List is still 10 rows');
        let modifyIndexes = Array.from(rows).map((row) =>
          parseInt(row.getAttribute('data-test-modify-index'))
        );
        assert.deepEqual(
          modifyIndexes,
          Array(10)
            .fill()
            .map((_, i) => i + 1)
            .reverse(),
          'Jobs are sorted by modify index'
        );

        // Create a new job
        server.create('job', {
          namespaceId: 'default',
          resourceSpec: Array(1).fill('M: 256, C: 500'),
          groupAllocCount: 1,
          modifyIndex: 11,
          createAllocations: false,
          shallow: true,
          name: 'new-job',
        });

        const controller = this.owner.lookup('controller:jobs.index');

        let currentParams = {
          per_page: 10,
        };

        // We have to wait for watchJobIDs to trigger the "dueling query" with watchJobs.
        // Since we can't await the watchJobs promise, we set a reasonably short timeout
        // to check the state of the list after the dueling query has completed.
        await controller.watchJobIDs.perform(currentParams, 0);

        let updatedUnshownJob = assert.async(); // watch for this to say "My tests oughta be passing by now"
        const duelingQueryUpdateTime = 200;

        assert.timeout(500);

        setTimeout(async () => {
          // Order should still be be 10-1
          rows = document.querySelectorAll('.job-row');
          modifyIndexes = Array.from(rows).map((row) =>
            parseInt(row.getAttribute('data-test-modify-index'))
          );
          assert.deepEqual(
            modifyIndexes,
            Array(10)
              .fill()
              .map((_, i) => i + 1)
              .reverse(),
            'Jobs are sorted by modify index, off-page job not showing up yet'
          );
          assert
            .dom('[data-test-updates-pending-button]')
            .exists('The refresh button is present');
          assert
            .dom('[data-test-pager="next"]')
            .isNotDisabled(
              'Next button is enabled in spite of the new job not showing up yet'
            );

          // Simulate one of the on-page jobs getting its modify-index bumped. It should remain in place.
          let existingJobToUpdate = server.db.jobs.findBy(
            (job) => job.modifyIndex === 5
          );
          server.db.jobs.update(existingJobToUpdate.id, { modifyIndex: 12 });
          await controller.watchJobIDs.perform(currentParams, 0);
          let updatedShownJob = assert.async();

          setTimeout(async () => {
            rows = document.querySelectorAll('.job-row');
            modifyIndexes = Array.from(rows).map((row) =>
              parseInt(row.getAttribute('data-test-modify-index'))
            );
            assert.deepEqual(
              modifyIndexes,
              [10, 9, 8, 7, 6, 12, 4, 3, 2, 1],
              'Jobs are sorted by modify index, on-page job remains in-place, and off-page pending'
            );
            assert
              .dom('[data-test-updates-pending-button]')
              .exists('The refresh button is still present');
            assert
              .dom('[data-test-pager="next"]')
              .isNotDisabled('Next button is still enabled');

            // Click the refresh button
            await click('[data-test-updates-pending-button]');
            rows = document.querySelectorAll('.job-row');
            modifyIndexes = Array.from(rows).map((row) =>
              parseInt(row.getAttribute('data-test-modify-index'))
            );
            assert.deepEqual(
              modifyIndexes,
              [12, 11, 10, 9, 8, 7, 6, 4, 3, 2],
              'Jobs are sorted by modify index, after refresh'
            );
            assert
              .dom('[data-test-updates-pending-button]')
              .doesNotExist('The refresh button is gone');
            updatedShownJob();
          }, duelingQueryUpdateTime);
          updatedUnshownJob();
        }, duelingQueryUpdateTime);

        localStorage.removeItem('nomadPageSize');
        localStorage.removeItem('nomadLiveUpdateJobsIndex');
      });
    });
  });

  module('Searching and Filtering', function () {
    module('Search', function () {
      test('Searching reasons about whether you intended a job name or a filter expression', async function (assert) {
        localStorage.setItem('nomadPageSize', '10');
        createJobs(server, 10);
        await JobsList.visit();

        await JobsList.search.fillIn('something-that-surely-doesnt-exist');
        // check to see that we fired off a request; check handledRequests to find one with a ?filter in it
        assert.ok(
          server.pretender.handledRequests.find((req) =>
            decodeURIComponent(req.url).includes(
              '?filter=Name contains "something-that-surely-doesnt-exist"'
            )
          ),
          'A request was made with a filter query param that assumed job name'
        );

        await JobsList.search.fillIn('Namespace == ns-2');

        assert.ok(
          server.pretender.handledRequests.find((req) =>
            decodeURIComponent(req.url).includes('?filter=Namespace == ns-2')
          ),
          'A request was made with a filter query param for a filter expression as typed'
        );

        localStorage.removeItem('nomadPageSize');
      });

      test('Searching by name filters the list', async function (assert) {
        localStorage.setItem('nomadPageSize', '10');
        createJobs(server, 10);
        server.create('job', {
          name: 'hashi-one',
          id: 'hashi-one',
          modifyIndex: 0,
        });
        server.create('job', {
          name: 'hashi-two',
          id: 'hashi-two',
          modifyIndex: 0,
        });
        await JobsList.visit();

        assert
          .dom('.job-row')
          .exists(
            { count: 10 },
            'Initially, 10 jobs are listed without any filters.'
          );
        assert
          .dom('[data-test-job-row="hashi-one"]')
          .doesNotExist(
            'The specific job hashi-one should not appear without filtering.'
          );
        assert
          .dom('[data-test-job-row="hashi-two"]')
          .doesNotExist(
            'The specific job hashi-two should also not appear without filtering.'
          );

        await JobsList.search.fillIn('hashi-one');
        assert
          .dom('.job-row')
          .exists(
            { count: 1 },
            'Only one job should be visible when filtering by the name "hashi-one".'
          );
        assert
          .dom('[data-test-job-row="hashi-one"]')
          .exists(
            'The job hashi-one appears as expected when filtered by name.'
          );
        assert
          .dom('[data-test-job-row="hashi-two"]')
          .doesNotExist(
            'The job hashi-two should not appear when filtering by "hashi-one".'
          );

        await JobsList.search.fillIn('hashi');
        assert
          .dom('.job-row')
          .exists(
            { count: 2 },
            'Two jobs should appear when the filter "hashi" matches both job names.'
          );
        assert
          .dom('[data-test-job-row="hashi-one"]')
          .exists(
            'Job hashi-one is correctly displayed under the "hashi" filter.'
          );
        assert
          .dom('[data-test-job-row="hashi-two"]')
          .exists(
            'Job hashi-two is correctly displayed under the "hashi" filter.'
          );

        await JobsList.search.fillIn('Name == hashi');
        assert
          .dom('.job-row')
          .exists(
            { count: 0 },
            'No jobs should appear when an incorrect filter format "Name == hashi" is used.'
          );

        await JobsList.search.fillIn('');
        assert
          .dom('.job-row')
          .exists(
            { count: 10 },
            'All jobs reappear when the search filter is cleared.'
          );
        assert
          .dom('[data-test-job-row="hashi-one"]')
          .doesNotExist(
            'The job hashi-one should disappear again when the filter is cleared.'
          );
        assert
          .dom('[data-test-job-row="hashi-two"]')
          .doesNotExist(
            'The job hashi-two should disappear again when the filter is cleared.'
          );

        localStorage.removeItem('nomadPageSize');
      });

      test('Searching by type filters the list', async function (assert) {
        localStorage.setItem('nomadPageSize', '10');
        server.createList('job', 10, {
          createAllocations: false,
          type: 'service',
          modifyIndex: 10,
        });

        server.create('job', {
          id: 'batch-job',
          type: 'batch',
          createAllocations: false,
          modifyIndex: 9,
        });
        server.create('job', {
          id: 'system-job',
          type: 'system',
          createAllocations: false,
          modifyIndex: 9,
        });
        server.create('job', {
          id: 'sysbatch-job',
          type: 'sysbatch',
          createAllocations: false,
          modifyIndex: 9,
        });
        server.create('job', {
          id: 'sysbatch-job-2',
          type: 'sysbatch',
          createAllocations: false,
          modifyIndex: 9,
        });

        await JobsList.visit();
        assert
          .dom('.job-row')
          .exists(
            { count: 10 },
            'Initial setup should show 10 jobs of type "service".'
          );
        assert
          .dom('[data-test-job-type="service"]')
          .exists(
            { count: 10 },
            'All initial jobs are confirmed to be of type "service".'
          );

        await JobsList.search.fillIn('Type == batch');
        assert
          .dom('.job-row')
          .exists(
            { count: 1 },
            'Filtering by "Type == batch" should show exactly one job.'
          );
        assert
          .dom('[data-test-job-type="batch"]')
          .exists(
            { count: 1 },
            'The single job of type "batch" is displayed as expected.'
          );

        await JobsList.search.fillIn('Type == system');
        assert
          .dom('.job-row')
          .exists(
            { count: 1 },
            'Only one job should be displayed when filtering by "Type == system".'
          );
        assert
          .dom('[data-test-job-type="system"]')
          .exists(
            { count: 1 },
            'The job of type "system" appears as expected.'
          );

        await JobsList.search.fillIn('Type == sysbatch');
        assert
          .dom('.job-row')
          .exists(
            { count: 2 },
            'Two jobs should be visible under the filter "Type == sysbatch".'
          );
        assert
          .dom('[data-test-job-type="sysbatch"]')
          .exists(
            { count: 2 },
            'Both jobs of type "sysbatch" are correctly displayed.'
          );

        await JobsList.search.fillIn('Type contains sys');
        assert
          .dom('.job-row')
          .exists(
            { count: 3 },
            'Filter "Type contains sys" should show three jobs.'
          );
        assert
          .dom('[data-test-job-type="sysbatch"]')
          .exists(
            { count: 2 },
            'Two jobs of type "sysbatch" match the "sys" substring.'
          );
        assert
          .dom('[data-test-job-type="system"]')
          .exists(
            { count: 1 },
            'One job of type "system" matches the "sys" substring.'
          );

        await JobsList.search.fillIn('Type != service');
        assert
          .dom('.job-row')
          .exists(
            { count: 4 },
            'Four jobs should be visible when excluding type "service".'
          );
        assert
          .dom('[data-test-job-type="batch"]')
          .exists({ count: 1 }, 'One batch job is visible.');
        assert
          .dom('[data-test-job-type="system"]')
          .exists({ count: 1 }, 'One system job is visible.');
        assert
          .dom('[data-test-job-type="sysbatch"]')
          .exists({ count: 2 }, 'Two sysbatch jobs are visible.');

        // Next/Last buttons are disabled when searching for the 10 services bc there's just 10
        await JobsList.search.fillIn('Type == service');
        assert.dom('.job-row').exists({ count: 10 });
        assert.dom('[data-test-job-type="service"]').exists({ count: 10 });
        assert
          .dom('[data-test-pager="next"]')
          .isDisabled(
            'The next page button should be disabled when all jobs fit on one page.'
          );
        assert
          .dom('[data-test-pager="last"]')
          .isDisabled(
            'The last page button should also be disabled under the same conditions.'
          );

        // But if we disinclude sysbatch we'll have 12, so next/last should be clickable
        await JobsList.search.fillIn('Type != sysbatch');
        assert.dom('.job-row').exists({ count: 10 });
        assert
          .dom('[data-test-pager="next"]')
          .isNotDisabled(
            'The next page button should be enabled when not all jobs are shown on one page.'
          );
        assert
          .dom('[data-test-pager="last"]')
          .isNotDisabled('The last page button should be enabled as well.');

        localStorage.removeItem('nomadPageSize');
      });

      test('Searching with a bad filter expression gives hints', async function (assert) {
        localStorage.setItem('nomadPageSize', '10');
        createJobs(server, 10);
        await JobsList.visit();

        // Try with "type" instead of "Type"
        await JobsList.search.fillIn('type == foo');
        assert
          .dom('[data-test-empty-jobs-list]')
          .includesText(
            'No jobs match your current filter selection: type == foo'
          );
        assert.dom('[data-test-filter-correction]').exists();
        await percySnapshot(assert);

        await JobsList.search.fillIn('foo != bar');
        assert
          .dom('[data-test-empty-jobs-list]')
          .includesText('Did you mistype a key?');
        assert.dom('[data-test-filter-suggestion]').exists();
        await percySnapshot(assert);

        await JobsList.search.fillIn('Name == surelyDoesntExist');
        assert
          .dom('[data-test-empty-jobs-list]')
          .includesText(
            'No jobs match your current filter selection: Name == surelyDoesntExist'
          );
        assert.dom('[data-test-filter-random-suggestion]').exists();
        await percySnapshot(assert);

        localStorage.removeItem('nomadPageSize');
      });
    });
    module('Filtering', function () {
      test('Filtering by namespace filters the list', async function (assert) {
        localStorage.setItem('nomadPageSize', '10');

        server.create('namespace', {
          id: 'default',
          name: 'default',
        });

        server.create('namespace', {
          id: 'ns-2',
          name: 'ns-2',
        });

        server.createList('job', 10, {
          createAllocations: false,
          namespaceId: 'default',
          modifyIndex: 10,
        });

        server.create('job', {
          id: 'ns-2-job',
          namespaceId: 'ns-2',
          createAllocations: false,
          modifyIndex: 9,
        });

        // By default, start without a namespace filter applied
        await JobsList.visit();
        assert
          .dom('.job-row')
          .exists(
            { count: 10 },
            'Initial setup should show 10 jobs in the default namespace.'
          );
        assert
          .dom('[data-test-job-row="ns-2-job"]')
          .doesNotExist(
            'The job in the ns-2 namespace should not appear without filtering.'
          );

        assert
          .dom('[data-test-pager="next"]')
          .isNotDisabled(
            '11 jobs on "All" namespace, so second page is available'
          );

        // Toggle ns-2 namespace
        await JobsList.facets.namespace.toggle();
        await JobsList.facets.namespace.options[1].toggle();

        assert
          .dom('.job-row')
          .exists(
            { count: 1 },
            'Only one job should be visible when filtering by the ns-2 namespace.'
          );
        assert
          .dom('[data-test-job-row="ns-2-job"]')
          .exists(
            'The job in the ns-2 namespace appears as expected when filtered.'
          );

        // Switch to default namespace
        await JobsList.facets.namespace.options[1].toggle(); //ns-2 off
        await JobsList.facets.namespace.options[0].toggle(); //default on

        assert
          .dom('.job-row')
          .exists(
            { count: 10 },
            'All jobs reappear when the search filter is cleared.'
          );
        assert
          .dom('[data-test-job-row="ns-2-job"]')
          .doesNotExist(
            'The job in the ns-2 namespace should disappear when the filter is cleared.'
          );

        assert
          .dom('[data-test-pager="next"]')
          .isDisabled(
            '10 jobs in "Default" namespace, so second page is not available'
          );

        // Turn both on
        await JobsList.facets.namespace.options[1].toggle(); //ns-2 on, default was already on

        assert
          .dom('.job-row')
          .exists(
            { count: 10 },
            'Both-on should show 10 jobs in the default namespace.'
          );
        assert
          .dom('[data-test-job-row="ns-2-job"]')
          .doesNotExist(
            'The job in the ns-2 namespace should not appear on the first page.'
          );

        assert
          .dom('[data-test-pager="next"]')
          .isNotDisabled(
            '11 jobs with both namespaces filtered, so second page is available'
          );

        localStorage.removeItem('nomadPageSize');
      });
      test('Namespace filter options can be filtered', async function (assert) {
        localStorage.setItem('nomadPageSize', '10');
        server.create('namespace', {
          id: 'default',
          name: 'default',
        });
        server.create('namespace', {
          id: 'Bonderman',
          name: 'Bonderman',
        });
        server.create('namespace', {
          id: 'Robertson',
          name: 'Robertson',
        });
        server.create('namespace', {
          id: 'Rogers',
          name: 'Rogers',
        });
        server.create('namespace', {
          id: 'Verlander',
          name: 'Verlander',
        });
        server.create('namespace', {
          id: 'Miner',
          name: 'Miner',
        });
        server.createList('job', 3, {
          createAllocations: false,
          namespaceId: 'default',
          modifyIndex: 10,
        });
        server.createList('job', 3, {
          createAllocations: false,
          namespaceId: 'Bonderman',
          modifyIndex: 10,
        });
        server.createList('job', 2, {
          createAllocations: false,
          namespaceId: 'Verlander',
          modifyIndex: 10,
        });
        server.createList('job', 2, {
          createAllocations: false,
          namespaceId: 'Rogers',
          modifyIndex: 10,
        });
        await JobsList.visit();

        await JobsList.facets.namespace.toggle();
        assert.dom('[data-test-namespace-filter-searchbox]').exists();
        // and it should be focused
        assert.dom('[data-test-namespace-filter-searchbox]').isFocused();
        // and there should be 6 things there
        assert.dom('[data-test-dropdown-option]').exists({ count: 6 });
        await typeIn('[data-test-namespace-filter-searchbox]', 'Bonderman');
        assert.dom('[data-test-dropdown-option]').exists({ count: 1 });
        document.querySelector('[data-test-namespace-filter-searchbox]').value =
          ''; // clear
        await typeIn('[data-test-namespace-filter-searchbox]', 'n');
        assert.dom('[data-test-dropdown-option]').exists({ count: 4 });
        await percySnapshot(assert);
      });
      test('Namespace filter only shows up if the server has more than one namespace', async function (assert) {
        localStorage.setItem('nomadPageSize', '10');

        server.create('namespace', {
          id: 'default',
          name: 'default',
        });

        server.createList('job', 10, {
          createAllocations: false,
          namespaceId: 'default',
          modifyIndex: 10,
        });

        await JobsList.visit();
        assert
          .dom('[data-test-facet="Namespace"]')
          .doesNotExist(
            'Namespace filter should not appear with only one namespace.'
          );

        server.create('namespace', {
          id: 'Bonderman',
          name: 'Bonderman',
        });

        await visit('/clients'); // go to another page to force a full refresh
        await JobsList.visit();

        assert
          .dom('[data-test-facet="Namespace"]')
          .exists(
            'Namespace filter should appear with more than one namespace.'
          );

        localStorage.removeItem('nomadPageSize');
      });
      test('Filtering by status filters the list', async function (assert) {
        localStorage.setItem('nomadPageSize', '10');
        server.createList('job', 10, {
          createAllocations: false,
          status: 'running',
          modifyIndex: 10,
        });

        server.create('job', {
          id: 'pending-job',
          status: 'pending',
          createAllocations: false,
          modifyIndex: 9,
        });

        server.create('job', {
          id: 'dead-job',
          status: 'dead',
          createAllocations: false,
          modifyIndex: 8,
        });

        await JobsList.visit();
        assert
          .dom('.job-row')
          .exists(
            { count: 10 },
            'Initial setup should show 10 jobs in the "running" status.'
          );
        assert
          .dom('[data-test-job-row="pending-job"]')
          .doesNotExist(
            'The job in the "pending" status should not appear without filtering.'
          );
        assert
          .dom('[data-test-pager="next"]')
          .isNotDisabled(
            '10 jobs in "running" status, so second page is available'
          );

        await JobsList.facets.status.toggle();
        await JobsList.facets.status.options[0].toggle(); // pending

        assert
          .dom('.job-row')
          .exists(
            { count: 1 },
            'Only one job should be visible when filtering by the "pending" status.'
          );
        assert
          .dom('[data-test-job-row="pending-job"]')
          .exists(
            'The job in the "pending" status appears as expected when filtered.'
          );

        assert
          .dom('[data-test-pager="next"]')
          .isDisabled(
            '1 job in "pending" status, so second page is not available'
          );

        await JobsList.facets.status.options[2].toggle(); // dead
        assert
          .dom('.job-row')
          .exists(
            { count: 2 },
            'Two jobs should be visible when the "dead" filter is added'
          );
        assert
          .dom('[data-test-job-row="dead-job"]')
          .exists(
            { count: 1 },
            'The job in the "dead" status appears as expected when filtered.'
          );

        localStorage.removeItem('nomadPageSize');
      });

      test('Filtering by a dynamically-generated facet: data-test-facet="Node Pool"', async function (assert) {
        localStorage.setItem('nomadPageSize', '10');

        server.create('node-pool', {
          id: 'pool-1',
          name: 'pool-1',
        });
        server.create('node-pool', {
          id: 'pool-2',
          name: 'pool-2',
        });

        server.createList('job', 10, {
          createAllocations: false,
          nodePool: 'pool-1',
          modifyIndex: 10,
        });

        server.create('job', {
          id: 'pool-2-job',
          nodePool: 'pool-2',
          createAllocations: false,
          modifyIndex: 9,
        });

        await JobsList.visit();
        assert
          .dom('.job-row')
          .exists(
            { count: 10 },
            'Initial setup should show 10 jobs in the "pool-1" node pool.'
          );
        assert
          .dom('[data-test-job-row="pool-2-job"]')
          .doesNotExist(
            'The job in the "pool-2" node pool should not appear without filtering.'
          );
        await JobsList.facets.nodePool.toggle();

        await JobsList.facets.nodePool.options[2].toggle(); // pool-2
        assert
          .dom('.job-row')
          .exists(
            { count: 1 },
            'Only one job should be visible when filtering by the "pool-2" node pool.'
          );
        assert
          .dom('[data-test-job-row="pool-2-job"]')
          .exists(
            'The job in the "pool-2" node pool appears as expected when filtered.'
          );

        localStorage.removeItem('nomadPageSize');
      });

      test('Combined Filtering and Searching', async function (assert) {
        localStorage.setItem('nomadPageSize', '10');
        // 2 service, 1 batch, 1 system, 1 sysbatch
        // 3 running, 1 dead, 1 pending
        server.create('job', {
          id: 'job1',
          name: 'Alpha Processing',
          type: 'batch',
          status: 'running',
        });
        server.create('job', {
          id: 'job2',
          name: 'Beta Calculation',
          type: 'service',
          status: 'dead',
        });
        server.create('job', {
          id: 'job3',
          name: 'Gamma Analysis',
          type: 'sysbatch',
          status: 'pending',
        });
        server.create('job', {
          id: 'job4',
          name: 'Delta Research',
          type: 'system',
          status: 'running',
        });
        server.create('job', {
          id: 'job5',
          name: 'Epsilon Development',
          type: 'service',
          status: 'running',
        });

        // All 5 jobs show up by default
        await JobsList.visit();
        assert.dom('.job-row').exists({ count: 5 }, 'All 5 jobs are visible');

        // Toggle type to "service", should see 2 jobs
        await JobsList.facets.type.toggle();
        await JobsList.facets.type.options[1].toggle();
        assert
          .dom('.job-row')
          .exists({ count: 2 }, 'Two service jobs are visible');

        // additionally, enable "batch" type
        await JobsList.facets.type.options[0].toggle();
        assert
          .dom('.job-row')
          .exists(
            { count: 3 },
            'Three jobs are visible with service and batch types'
          );
        assert.dom('[data-test-job-row="job1"]').exists();
        assert.dom('[data-test-job-row="job2"]').exists();
        assert.dom('[data-test-job-row="job5"]').exists();

        // additionally, enable "running" status to filter down to just the running ones
        await JobsList.facets.status.toggle();
        await JobsList.facets.status.options[1].toggle();
        assert
          .dom('.job-row')
          .exists({ count: 2 }, 'Two running service/batch jobs are visible');
        assert.dom('[data-test-job-row="job1"]').exists();
        assert.dom('[data-test-job-row="job5"]').exists();
        assert.dom('[data-test-job-row="job2"]').doesNotExist();

        // additionally, perform a search for Name != "Alpha Processing"
        await JobsList.search.fillIn('Name != "Alpha Processing"');
        assert
          .dom('.job-row')
          .exists({ count: 1 }, 'One running service job is visible');
        assert.dom('[data-test-job-row="job5"]').exists();
        assert.dom('[data-test-job-row="job1"]').doesNotExist();
      });
    });
  });
});

/**
 *
 * @param {*} server
 * @param {number} jobsToCreate
 */
function createJobs(server, jobsToCreate) {
  for (let i = 0; i < jobsToCreate; i++) {
    server.create('job', {
      namespaceId: 'default',
      resourceSpec: Array(1).fill('M: 256, C: 500'),
      groupAllocCount: 1,
      modifyIndex: i + 1,
      createAllocations: false,
      shallow: true,
    });
  }
}

async function facetOptions(assert, beforeEach, facet, expectedOptions) {
  await beforeEach();
  await facet.toggle();

  let expectation;
  if (typeof expectedOptions === 'function') {
    expectation = expectedOptions(server.db.jobs);
  } else {
    expectation = expectedOptions;
  }

  assert.deepEqual(
    facet.options.map((option) => option.label.trim()),
    expectation,
    'Options for facet are as expected'
  );
}

function testFacet(
  label,
  { facet, paramName, beforeEach, filter, expectedOptions, dynamicStrings }
) {
  test(`the ${label} facet has the correct options`, async function (assert) {
    await facetOptions(assert, beforeEach, facet, expectedOptions);
  });

  test(`the ${label} facet filters the jobs list by ${label}`, async function (assert) {
    let option;

    await beforeEach();
    await facet.toggle();

    option = facet.options.objectAt(0);
    await option.toggle();

    const selection = [option.label];
    const expectedJobs = server.db.jobs
      .filter((job) => filter(job, selection))
      .sortBy('modifyIndex')
      .reverse();

    JobsList.jobs.forEach((job, index) => {
      assert.equal(
        job.id,
        expectedJobs[index].id,
        `Job at ${index} is ${expectedJobs[index].id}`
      );
    });
  });

  test(`selecting multiple options in the ${label} facet results in a broader search`, async function (assert) {
    const selection = [];

    await beforeEach();
    await facet.toggle();

    const option1 = facet.options.objectAt(0);
    const option2 = facet.options.objectAt(1);
    await option1.toggle();
    selection.push(option1.label);
    await option2.toggle();
    selection.push(option2.label);

    const expectedJobs = server.db.jobs
      .filter((job) => filter(job, selection))
      .sortBy('modifyIndex')
      .reverse();

    JobsList.jobs.forEach((job, index) => {
      assert.equal(
        job.id,
        expectedJobs[index].id,
        `Job at ${index} is ${expectedJobs[index].id}`
      );
    });
  });

  test(`selecting options in the ${label} facet updates the ${paramName} query param`, async function (assert) {
    const selection = [];

    await beforeEach();
    await facet.toggle();

    const option1 = facet.options.objectAt(0);
    const option2 = facet.options.objectAt(1);
    await option1.toggle();
    selection.push(option1.label);
    await option2.toggle();
    selection.push(option2.label);

    selection.forEach((selection) => {
      let capitalizedParamName =
        paramName.charAt(0).toUpperCase() + paramName.slice(1);
      // allowing for the possibility of "-" or other characters in the string, we wrap the filter parameter in quotes for namespaces and node pools
      assert.ok(
        currentURL().includes(
          dynamicStrings
            ? encodeURIComponent(`${capitalizedParamName} == "${selection}"`)
            : encodeURIComponent(`${capitalizedParamName} == ${selection}`)
        ),
        `URL has the correct query param key and value for ${selection}`
      );
    });
  });
}
