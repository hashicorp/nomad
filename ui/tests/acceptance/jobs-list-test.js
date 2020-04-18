import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import pageSizeSelect from './behaviors/page-size-select';
import JobsList from 'nomad-ui/tests/pages/jobs/list';

let managementToken, clientToken;

module('Acceptance | jobs list', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function() {
    // Required for placing allocations (a result of creating jobs)
    server.create('node');

    managementToken = server.create('token');
    clientToken = server.create('token');

    window.localStorage.clear();
    window.localStorage.nomadTokenSecret = managementToken.secretId;
  });

  test('visiting /jobs', async function(assert) {
    await JobsList.visit();

    assert.equal(currentURL(), '/jobs');
    assert.equal(document.title, 'Jobs - Nomad');
  });

  test('/jobs should list the first page of jobs sorted by modify index', async function(assert) {
    const jobsCount = JobsList.pageSize + 1;
    server.createList('job', jobsCount, { createAllocations: false });

    await JobsList.visit();

    const sortedJobs = server.db.jobs.sortBy('modifyIndex').reverse();
    assert.equal(JobsList.jobs.length, JobsList.pageSize);
    JobsList.jobs.forEach((job, index) => {
      assert.equal(job.name, sortedJobs[index].name, 'Jobs are ordered');
    });
  });

  test('each job row should contain information about the job', async function(assert) {
    server.createList('job', 2);
    const job = server.db.jobs.sortBy('modifyIndex').reverse()[0];
    const taskGroups = server.db.taskGroups.where({ jobId: job.id });

    await JobsList.visit();

    const jobRow = JobsList.jobs.objectAt(0);

    assert.equal(jobRow.name, job.name, 'Name');
    assert.equal(jobRow.link, `/ui/jobs/${job.id}`, 'Detail Link');
    assert.equal(jobRow.status, job.status, 'Status');
    assert.equal(jobRow.type, typeForJob(job), 'Type');
    assert.equal(jobRow.priority, job.priority, 'Priority');
    assert.equal(jobRow.taskGroups, taskGroups.length, '# Groups');
  });

  test('each job row should link to the corresponding job', async function(assert) {
    server.create('job');
    const job = server.db.jobs[0];

    await JobsList.visit();
    await JobsList.jobs.objectAt(0).clickName();

    assert.equal(currentURL(), `/jobs/${job.id}`);
  });

  test('the new job button transitions to the new job page', async function(assert) {
    await JobsList.visit();
    await JobsList.runJobButton.click();

    assert.equal(currentURL(), '/jobs/run');
  });

  test('the job run button is disabled when the token lacks permission', async function(assert) {
    window.localStorage.nomadTokenSecret = clientToken.secretId;
    await JobsList.visit();

    assert.ok(JobsList.runJobButton.isDisabled);

    await JobsList.runJobButton.click();
    assert.equal(currentURL(), '/jobs');
  });

  test('the job run button state can change between namespaces', async function(assert) {
    server.createList('namespace', 2);
    const job1 = server.create('job', { namespaceId: server.db.namespaces[0].id });
    const job2 = server.create('job', { namespaceId: server.db.namespaces[1].id });

    window.localStorage.nomadTokenSecret = clientToken.secretId;

    const policy = server.create('policy', {
      id: 'something',
      name: 'something',
      rulesJSON: {
        Namespaces: [
          {
            Name: job1.namespaceId,
            Capabilities: ['list-jobs', 'submit-job'],
          },
          {
            Name: job2.namespaceId,
            Capabilities: ['list-jobs'],
          },
        ],
      },
    });

    clientToken.policyIds = [policy.id];
    clientToken.save();

    await JobsList.visit();
    assert.notOk(JobsList.runJobButton.isDisabled);

    const secondNamespace = server.db.namespaces[1];
    await JobsList.visit({ namespace: secondNamespace.id });
    assert.ok(JobsList.runJobButton.isDisabled);
  });

  test('the anonymous policy is fetched to check whether to show the job run button', async function(assert) {
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

  test('when there are no jobs, there is an empty message', async function(assert) {
    await JobsList.visit();

    assert.ok(JobsList.isEmpty, 'There is an empty message');
    assert.equal(JobsList.emptyState.headline, 'No Jobs', 'The message is appropriate');
  });

  test('when there are jobs, but no matches for a search result, there is an empty message', async function(assert) {
    server.create('job', { name: 'cat 1' });
    server.create('job', { name: 'cat 2' });

    await JobsList.visit();

    await JobsList.search('dog');
    assert.ok(JobsList.isEmpty, 'The empty message is shown');
    assert.equal(JobsList.emptyState.headline, 'No Matches', 'The message is appropriate');
  });

  test('searching resets the current page', async function(assert) {
    server.createList('job', JobsList.pageSize + 1, { createAllocations: false });

    await JobsList.visit();
    await JobsList.nextPage();

    assert.equal(currentURL(), '/jobs?page=2', 'Page query param captures page=2');

    await JobsList.search('foobar');

    assert.equal(currentURL(), '/jobs?search=foobar', 'No page query param');
  });

  test('when the namespace query param is set, only matching jobs are shown and the namespace value is forwarded to app state', async function(assert) {
    server.createList('namespace', 2);
    const job1 = server.create('job', { namespaceId: server.db.namespaces[0].id });
    const job2 = server.create('job', { namespaceId: server.db.namespaces[1].id });

    await JobsList.visit();

    assert.equal(JobsList.jobs.length, 1, 'One job in the default namespace');
    assert.equal(JobsList.jobs.objectAt(0).name, job1.name, 'The correct job is shown');

    const secondNamespace = server.db.namespaces[1];
    await JobsList.visit({ namespace: secondNamespace.id });

    assert.equal(JobsList.jobs.length, 1, `One job in the ${secondNamespace.name} namespace`);
    assert.equal(JobsList.jobs.objectAt(0).name, job2.name, 'The correct job is shown');
  });

  test('when accessing jobs is forbidden, show a message with a link to the tokens page', async function(assert) {
    server.pretender.get('/v1/jobs', () => [403, {}, null]);

    await JobsList.visit();
    assert.equal(JobsList.error.title, 'Not Authorized');

    await JobsList.error.seekHelp();
    assert.equal(currentURL(), '/settings/tokens');
  });

  function typeForJob(job) {
    return job.periodic ? 'periodic' : job.parameterized ? 'parameterized' : job.type;
  }

  test('the jobs list page has appropriate faceted search options', async function(assert) {
    await JobsList.visit();

    assert.ok(JobsList.facets.type.isPresent, 'Type facet found');
    assert.ok(JobsList.facets.status.isPresent, 'Status facet found');
    assert.ok(JobsList.facets.datacenter.isPresent, 'Datacenter facet found');
    assert.ok(JobsList.facets.prefix.isPresent, 'Prefix facet found');
  });

  testFacet('Type', {
    facet: JobsList.facets.type,
    paramName: 'type',
    expectedOptions: ['Batch', 'Parameterized', 'Periodic', 'Service', 'System'],
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
      server.createList('job', 2, { createAllocations: false, type: 'service' });
      await JobsList.visit();
    },
    filter(job, selection) {
      let displayType = job.type;
      if (job.parameterized) displayType = 'parameterized';
      if (job.periodic) displayType = 'periodic';
      return selection.includes(displayType);
    },
  });

  testFacet('Status', {
    facet: JobsList.facets.status,
    paramName: 'status',
    expectedOptions: ['Pending', 'Running', 'Dead'],
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
      server.createList('job', 2, { status: 'dead', createAllocations: false, childrenCount: 0 });
      await JobsList.visit();
    },
    filter: (job, selection) => selection.includes(job.status),
  });

  testFacet('Datacenter', {
    facet: JobsList.facets.datacenter,
    paramName: 'dc',
    expectedOptions(jobs) {
      const allDatacenters = new Set(
        jobs.mapBy('datacenters').reduce((acc, val) => acc.concat(val), [])
      );
      return Array.from(allDatacenters).sort();
    },
    async beforeEach() {
      server.create('job', {
        datacenters: ['pdx', 'lax'],
        createAllocations: false,
        childrenCount: 0,
      });
      server.create('job', {
        datacenters: ['pdx', 'ord'],
        createAllocations: false,
        childrenCount: 0,
      });
      server.create('job', {
        datacenters: ['lax', 'jfk'],
        createAllocations: false,
        childrenCount: 0,
      });
      server.create('job', {
        datacenters: ['jfk', 'dfw'],
        createAllocations: false,
        childrenCount: 0,
      });
      server.create('job', { datacenters: ['pdx'], createAllocations: false, childrenCount: 0 });
      await JobsList.visit();
    },
    filter: (job, selection) => job.datacenters.find(dc => selection.includes(dc)),
  });

  testFacet('Prefix', {
    facet: JobsList.facets.prefix,
    paramName: 'prefix',
    expectedOptions: ['hashi (3)', 'nmd (2)', 'pre (2)'],
    async beforeEach() {
      [
        'pre-one',
        'hashi_one',
        'nmd.one',
        'one-alone',
        'pre_two',
        'hashi.two',
        'hashi-three',
        'nmd_two',
        'noprefix',
      ].forEach(name => {
        server.create('job', { name, createAllocations: false, childrenCount: 0 });
      });
      await JobsList.visit();
    },
    filter: (job, selection) => selection.find(prefix => job.name.startsWith(prefix)),
  });

  test('when the facet selections result in no matches, the empty state states why', async function(assert) {
    server.createList('job', 2, { status: 'pending', createAllocations: false, childrenCount: 0 });

    await JobsList.visit();

    await JobsList.facets.status.toggle();
    await JobsList.facets.status.options.objectAt(1).toggle();
    assert.ok(JobsList.isEmpty, 'There is an empty message');
    assert.equal(JobsList.emptyState.headline, 'No Matches', 'The message is appropriate');
  });

  test('the jobs list is immediately filtered based on query params', async function(assert) {
    server.create('job', { type: 'batch', createAllocations: false });
    server.create('job', { type: 'service', createAllocations: false });

    await JobsList.visit({ type: JSON.stringify(['batch']) });

    assert.equal(JobsList.jobs.length, 1, 'Only one job shown due to query param');
  });

  pageSizeSelect({
    resourceName: 'job',
    pageObject: JobsList,
    pageObjectList: JobsList.jobs,
    async setup() {
      server.createList('job', JobsList.pageSize, { shallow: true, createAllocations: false });
      await JobsList.visit();
    },
  });

  function testFacet(label, { facet, paramName, beforeEach, filter, expectedOptions }) {
    test(`the ${label} facet has the correct options`, async function(assert) {
      await beforeEach();
      await facet.toggle();

      let expectation;
      if (typeof expectedOptions === 'function') {
        expectation = expectedOptions(server.db.jobs);
      } else {
        expectation = expectedOptions;
      }

      assert.deepEqual(
        facet.options.map(option => option.label.trim()),
        expectation,
        'Options for facet are as expected'
      );
    });

    test(`the ${label} facet filters the jobs list by ${label}`, async function(assert) {
      let option;

      await beforeEach();
      await facet.toggle();

      option = facet.options.objectAt(0);
      await option.toggle();

      const selection = [option.key];
      const expectedJobs = server.db.jobs
        .filter(job => filter(job, selection))
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

    test(`selecting multiple options in the ${label} facet results in a broader search`, async function(assert) {
      const selection = [];

      await beforeEach();
      await facet.toggle();

      const option1 = facet.options.objectAt(0);
      const option2 = facet.options.objectAt(1);
      await option1.toggle();
      selection.push(option1.key);
      await option2.toggle();
      selection.push(option2.key);

      const expectedJobs = server.db.jobs
        .filter(job => filter(job, selection))
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

    test(`selecting options in the ${label} facet updates the ${paramName} query param`, async function(assert) {
      const selection = [];

      await beforeEach();
      await facet.toggle();

      const option1 = facet.options.objectAt(0);
      const option2 = facet.options.objectAt(1);
      await option1.toggle();
      selection.push(option1.key);
      await option2.toggle();
      selection.push(option2.key);

      assert.equal(
        currentURL(),
        `/jobs?${paramName}=${encodeURIComponent(JSON.stringify(selection))}`,
        'URL has the correct query param key and value'
      );
    });
  }
});
