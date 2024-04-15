/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/require-expect */
import { currentURL, click } from '@ember/test-helpers';
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
    assert.equal(jobRow.priority, job.priority, 'Priority');
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

  // TODO: Jobs list search
  test.skip('when there are jobs, but no matches for a search result, there is an empty message', async function (assert) {
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

  // TODO: Jobs list search
  test.skip('searching resets the current page', async function (assert) {
    server.createList('job', JobsList.pageSize + 1, {
      createAllocations: false,
    });

    await JobsList.visit();
    await JobsList.nextPage();

    assert.equal(
      currentURL(),
      '/jobs?page=2',
      'Page query param captures page=2'
    );

    await JobsList.search.fillIn('foobar');

    assert.equal(currentURL(), '/jobs?search=foobar', 'No page query param');
  });

  // TODO: Jobs list search
  test.skip('Search order overrides Sort order', async function (assert) {
    server.create('job', { name: 'car', modifyIndex: 1, priority: 200 });
    server.create('job', { name: 'cat', modifyIndex: 2, priority: 150 });
    server.create('job', { name: 'dog', modifyIndex: 3, priority: 100 });
    server.create('job', { name: 'dot', modifyIndex: 4, priority: 50 });

    await JobsList.visit();

    // Expect list to be in reverse modifyIndex order by default
    assert.equal(JobsList.jobs.objectAt(0).name, 'dot');
    assert.equal(JobsList.jobs.objectAt(1).name, 'dog');
    assert.equal(JobsList.jobs.objectAt(2).name, 'cat');
    assert.equal(JobsList.jobs.objectAt(3).name, 'car');

    // When sorting by name, expect list to be in alphabetical order
    await click('[data-test-sort-by="name"]'); // sorts desc
    await click('[data-test-sort-by="name"]'); // sorts asc

    assert.equal(JobsList.jobs.objectAt(0).name, 'car');
    assert.equal(JobsList.jobs.objectAt(1).name, 'cat');
    assert.equal(JobsList.jobs.objectAt(2).name, 'dog');
    assert.equal(JobsList.jobs.objectAt(3).name, 'dot');

    // When searching, the "name" sort is locked in. Fuzzy results for cat return both car and cat, but cat first.
    await JobsList.search.fillIn('cat');
    assert.equal(JobsList.jobs.length, 2);
    assert.equal(JobsList.jobs.objectAt(0).name, 'cat'); // higher fuzzy
    assert.equal(JobsList.jobs.objectAt(1).name, 'car');

    // Clicking priority sorter will maintain the search filter, but change the order
    await click('[data-test-sort-by="priority"]'); // sorts desc
    assert.equal(JobsList.jobs.objectAt(0).name, 'car'); // higher priority first
    assert.equal(JobsList.jobs.objectAt(1).name, 'cat');

    // Modifying search again will prioritize search "fuzzy" order
    await JobsList.search.fillIn(''); // trigger search reset
    await JobsList.search.fillIn('cat');
    assert.equal(JobsList.jobs.objectAt(0).name, 'cat'); // higher fuzzy
    assert.equal(JobsList.jobs.objectAt(1).name, 'car');
  });

  // TODO: Jobs list search
  test.skip('when a cluster has namespaces, each job row includes the job namespace', async function (assert) {
    server.createList('namespace', 2);
    server.createList('job', 2);
    const job = server.db.jobs.sortBy('modifyIndex').reverse()[0];

    await JobsList.visit({ namespace: '*' });

    const jobRow = JobsList.jobs.objectAt(0);
    assert.equal(jobRow.namespace, job.namespaceId);
  });

  // TODO: Jobs list filter
  test.skip('when the namespace query param is set, only matching jobs are shown', async function (assert) {
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
    await JobsList.visit({ namespace: firstNamespace.id });
    assert.equal(JobsList.jobs.length, 1, 'One job in the default namespace');
    assert.equal(
      JobsList.jobs.objectAt(0).name,
      job1.name,
      'The correct job is shown'
    );

    const secondNamespace = server.db.namespaces[1];
    await JobsList.visit({ namespace: secondNamespace.id });

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

  function typeForJob(job) {
    return job.periodic
      ? 'periodic'
      : job.parameterized
      ? 'parameterized'
      : job.type;
  }

  // TODO: Jobs list filter
  test.skip('the jobs list page has appropriate faceted search options', async function (assert) {
    await JobsList.visit();

    assert.ok(
      JobsList.facets.namespace.isHidden,
      'Namespace facet not found (no namespaces)'
    );
    assert.ok(JobsList.facets.type.isPresent, 'Type facet found');
    assert.ok(JobsList.facets.status.isPresent, 'Status facet found');
    assert.ok(JobsList.facets.datacenter.isPresent, 'Datacenter facet found');
    assert.ok(JobsList.facets.prefix.isPresent, 'Prefix facet found');
  });

  // TODO: Jobs list filter

  // testSingleSelectFacet('Namespace', {
  //   facet: JobsList.facets.namespace,
  //   paramName: 'namespace',
  //   expectedOptions: ['All (*)', 'default', 'namespace-2'],
  //   optionToSelect: 'namespace-2',
  //   async beforeEach() {
  //     server.create('namespace', { id: 'default' });
  //     server.create('namespace', { id: 'namespace-2' });
  //     server.createList('job', 2, { namespaceId: 'default' });
  //     server.createList('job', 2, { namespaceId: 'namespace-2' });
  //     await JobsList.visit();
  //   },
  //   filter(job, selection) {
  //     return job.namespaceId === selection;
  //   },
  // });

  // testFacet('Type', {
  //   facet: JobsList.facets.type,
  //   paramName: 'type',
  //   expectedOptions: [
  //     'Batch',
  //     'Pack',
  //     'Parameterized',
  //     'Periodic',
  //     'Service',
  //     'System',
  //     'System Batch',
  //   ],
  //   async beforeEach() {
  //     server.createList('job', 2, { createAllocations: false, type: 'batch' });
  //     server.createList('job', 2, {
  //       createAllocations: false,
  //       type: 'batch',
  //       periodic: true,
  //       childrenCount: 0,
  //     });
  //     server.createList('job', 2, {
  //       createAllocations: false,
  //       type: 'batch',
  //       parameterized: true,
  //       childrenCount: 0,
  //     });
  //     server.createList('job', 2, {
  //       createAllocations: false,
  //       type: 'service',
  //     });
  //     await JobsList.visit();
  //   },
  //   filter(job, selection) {
  //     let displayType = job.type;
  //     if (job.parameterized) displayType = 'parameterized';
  //     if (job.periodic) displayType = 'periodic';
  //     return selection.includes(displayType);
  //   },
  // });

  // testFacet('Status', {
  //   facet: JobsList.facets.status,
  //   paramName: 'status',
  //   expectedOptions: ['Pending', 'Running', 'Dead'],
  //   async beforeEach() {
  //     server.createList('job', 2, {
  //       status: 'pending',
  //       createAllocations: false,
  //       childrenCount: 0,
  //     });
  //     server.createList('job', 2, {
  //       status: 'running',
  //       createAllocations: false,
  //       childrenCount: 0,
  //     });
  //     server.createList('job', 2, {
  //       status: 'dead',
  //       createAllocations: false,
  //       childrenCount: 0,
  //     });
  //     await JobsList.visit();
  //   },
  //   filter: (job, selection) => selection.includes(job.status),
  // });

  // testFacet('Datacenter', {
  //   facet: JobsList.facets.datacenter,
  //   paramName: 'dc',
  //   expectedOptions(jobs) {
  //     const allDatacenters = new Set(
  //       jobs.mapBy('datacenters').reduce((acc, val) => acc.concat(val), [])
  //     );
  //     return Array.from(allDatacenters).sort();
  //   },
  //   async beforeEach() {
  //     server.create('job', {
  //       datacenters: ['pdx', 'lax'],
  //       createAllocations: false,
  //       childrenCount: 0,
  //     });
  //     server.create('job', {
  //       datacenters: ['pdx', 'ord'],
  //       createAllocations: false,
  //       childrenCount: 0,
  //     });
  //     server.create('job', {
  //       datacenters: ['lax', 'jfk'],
  //       createAllocations: false,
  //       childrenCount: 0,
  //     });
  //     server.create('job', {
  //       datacenters: ['jfk', 'dfw'],
  //       createAllocations: false,
  //       childrenCount: 0,
  //     });
  //     server.create('job', {
  //       datacenters: ['pdx'],
  //       createAllocations: false,
  //       childrenCount: 0,
  //     });
  //     await JobsList.visit();
  //   },
  //   filter: (job, selection) =>
  //     job.datacenters.find((dc) => selection.includes(dc)),
  // });

  // testFacet('Prefix', {
  //   facet: JobsList.facets.prefix,
  //   paramName: 'prefix',
  //   expectedOptions: ['hashi (3)', 'nmd (2)', 'pre (2)'],
  //   async beforeEach() {
  //     [
  //       'pre-one',
  //       'hashi_one',
  //       'nmd.one',
  //       'one-alone',
  //       'pre_two',
  //       'hashi.two',
  //       'hashi-three',
  //       'nmd_two',
  //       'noprefix',
  //     ].forEach((name) => {
  //       server.create('job', {
  //         name,
  //         createAllocations: false,
  //         childrenCount: 0,
  //       });
  //     });
  //     await JobsList.visit();
  //   },
  //   filter: (job, selection) =>
  //     selection.find((prefix) => job.name.startsWith(prefix)),
  // });

  // TODO: Jobs list filter
  test.skip('when the facet selections result in no matches, the empty state states why', async function (assert) {
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

  // TODO: Jobs list filter
  test.skip('the jobs list is immediately filtered based on query params', async function (assert) {
    server.create('job', { type: 'batch', createAllocations: false });
    server.create('job', { type: 'service', createAllocations: false });

    await JobsList.visit({ type: JSON.stringify(['batch']) });

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

  // async function facetOptions(assert, beforeEach, facet, expectedOptions) {
  //   await beforeEach();
  //   await facet.toggle();

  //   let expectation;
  //   if (typeof expectedOptions === 'function') {
  //     expectation = expectedOptions(server.db.jobs);
  //   } else {
  //     expectation = expectedOptions;
  //   }

  //   assert.deepEqual(
  //     facet.options.map((option) => option.label.trim()),
  //     expectation,
  //     'Options for facet are as expected'
  //   );
  // }

  // function testSingleSelectFacet(
  //   label,
  //   { facet, paramName, beforeEach, filter, expectedOptions, optionToSelect }
  // ) {
  //   test.skip(`the ${label} facet has the correct options`, async function (assert) {
  //     await facetOptions(assert, beforeEach, facet, expectedOptions);
  //   });

  //   test.skip(`the ${label} facet filters the jobs list by ${label}`, async function (assert) {
  //     await beforeEach();
  //     await facet.toggle();

  //     const option = facet.options.findOneBy('label', optionToSelect);
  //     const selection = option.key;
  //     await option.select();

  //     const expectedJobs = server.db.jobs
  //       .filter((job) => filter(job, selection))
  //       .sortBy('modifyIndex')
  //       .reverse();

  //     JobsList.jobs.forEach((job, index) => {
  //       assert.equal(
  //         job.id,
  //         expectedJobs[index].id,
  //         `Job at ${index} is ${expectedJobs[index].id}`
  //       );
  //     });
  //   });

  //   test.skip(`selecting an option in the ${label} facet updates the ${paramName} query param`, async function (assert) {
  //     await beforeEach();
  //     await facet.toggle();

  //     const option = facet.options.objectAt(1);
  //     const selection = option.key;
  //     await option.select();

  //     assert.ok(
  //       currentURL().includes(`${paramName}=${selection}`),
  //       'URL has the correct query param key and value'
  //     );
  //   });
  // }

  // function testFacet(
  //   label,
  //   { facet, paramName, beforeEach, filter, expectedOptions }
  // ) {
  //   test.skip(`the ${label} facet has the correct options`, async function (assert) {
  //     await facetOptions(assert, beforeEach, facet, expectedOptions);
  //   });

  //   test.skip(`the ${label} facet filters the jobs list by ${label}`, async function (assert) {
  //     let option;

  //     await beforeEach();
  //     await facet.toggle();

  //     option = facet.options.objectAt(0);
  //     await option.toggle();

  //     const selection = [option.key];
  //     const expectedJobs = server.db.jobs
  //       .filter((job) => filter(job, selection))
  //       .sortBy('modifyIndex')
  //       .reverse();

  //     JobsList.jobs.forEach((job, index) => {
  //       assert.equal(
  //         job.id,
  //         expectedJobs[index].id,
  //         `Job at ${index} is ${expectedJobs[index].id}`
  //       );
  //     });
  //   });

  //   test.skip(`selecting multiple options in the ${label} facet results in a broader search`, async function (assert) {
  //     const selection = [];

  //     await beforeEach();
  //     await facet.toggle();

  //     const option1 = facet.options.objectAt(0);
  //     const option2 = facet.options.objectAt(1);
  //     await option1.toggle();
  //     selection.push(option1.key);
  //     await option2.toggle();
  //     selection.push(option2.key);

  //     const expectedJobs = server.db.jobs
  //       .filter((job) => filter(job, selection))
  //       .sortBy('modifyIndex')
  //       .reverse();

  //     JobsList.jobs.forEach((job, index) => {
  //       assert.equal(
  //         job.id,
  //         expectedJobs[index].id,
  //         `Job at ${index} is ${expectedJobs[index].id}`
  //       );
  //     });
  //   });

  //   test.skip(`selecting options in the ${label} facet updates the ${paramName} query param`, async function (assert) {
  //     const selection = [];

  //     await beforeEach();
  //     await facet.toggle();

  //     const option1 = facet.options.objectAt(0);
  //     const option2 = facet.options.objectAt(1);
  //     await option1.toggle();
  //     selection.push(option1.key);
  //     await option2.toggle();
  //     selection.push(option2.key);

  //     assert.ok(
  //       currentURL().includes(encodeURIComponent(JSON.stringify(selection))),
  //       'URL has the correct query param key and value'
  //     );
  //   });

  //   test.skip('the run job button works when filters are set', async function (assert) {
  //     ['pre-one', 'pre-two', 'pre-three'].forEach((name) => {
  //       server.create('job', {
  //         name,
  //         createAllocations: false,
  //         childrenCount: 0,
  //       });
  //     });

  //     await JobsList.visit();

  //     await JobsList.facets.prefix.toggle();
  //     await JobsList.facets.prefix.options[0].toggle();

  //     await JobsList.runJobButton.click();
  //     assert.equal(currentURL(), '/jobs/run');
  //   });
  // }

  module('Pagination', function (hooks) {
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

        // Click next again, should be the last page, and therefore fewer than pageSize jobs
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

        localStorage.removeItem('nomadPageSize');
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
