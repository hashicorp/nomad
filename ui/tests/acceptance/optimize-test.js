/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/require-expect */
/* eslint-disable qunit/no-conditional-assertions */
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { currentURL, visit } from '@ember/test-helpers';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Response from 'ember-cli-mirage/response';
import moment from 'moment';
import { formatBytes, formatHertz, replaceMinus } from 'nomad-ui/utils/units';

import Optimize from 'nomad-ui/tests/pages/optimize';
import Layout from 'nomad-ui/tests/pages/layout';
import JobsList from 'nomad-ui/tests/pages/jobs/list';
import collapseWhitespace from '../helpers/collapse-whitespace';

let managementToken, clientToken;

function getLatestRecommendationSubmitTimeForJob(job) {
  const tasks = job.taskGroups.models
    .mapBy('tasks.models')
    .reduce((tasks, taskModels) => tasks.concat(taskModels), []);
  const recommendations = tasks.reduce(
    (recommendations, task) =>
      recommendations.concat(task.recommendations.models),
    []
  );
  return Math.max(...recommendations.mapBy('submitTime'));
}

module('Acceptance | optimize', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    server.create('feature', { name: 'Dynamic Application Sizing' });

    server.create('node-pool');
    server.create('node');

    server.createList('namespace', 2);

    const jobs = server.createList('job', 2, {
      createRecommendations: true,
      groupsCount: 1,
      groupTaskCount: 2,
      namespaceId: server.db.namespaces[1].id,
    });

    jobs.sort((jobA, jobB) => {
      return (
        getLatestRecommendationSubmitTimeForJob(jobB) -
        getLatestRecommendationSubmitTimeForJob(jobA)
      );
    });

    [this.job1, this.job2] = jobs;

    managementToken = server.create('token');
    clientToken = server.create('token');

    window.localStorage.clear();
    window.localStorage.nomadTokenSecret = managementToken.secretId;
  });

  test('it passes an accessibility audit', async function (assert) {
    await Optimize.visit();
    await a11yAudit(assert);
  });

  test('lets recommendations be toggled, reports the choices to the recommendations API, and displays task group recommendations serially', async function (assert) {
    const currentTaskGroup = this.job1.taskGroups.models[0];
    const nextTaskGroup = this.job2.taskGroups.models[0];

    const currentTaskGroupHasCPURecommendation = currentTaskGroup.tasks.models
      .mapBy('recommendations.models')
      .flat()
      .find((r) => r.resource === 'CPU');

    const currentTaskGroupHasMemoryRecommendation =
      currentTaskGroup.tasks.models
        .mapBy('recommendations.models')
        .flat()
        .find((r) => r.resource === 'MemoryMB');

    // If no CPU recommendation, will not be able to accept recommendation with all memory recommendations turned off

    if (!currentTaskGroupHasCPURecommendation) {
      const currentTaskGroupTask = currentTaskGroup.tasks.models[0];
      this.server.create('recommendation', {
        task: currentTaskGroupTask,
        resource: 'CPU',
      });
    }
    if (!currentTaskGroupHasMemoryRecommendation) {
      const currentTaskGroupTask = currentTaskGroup.tasks.models[0];
      this.server.create('recommendation', {
        task: currentTaskGroupTask,
        resource: 'MemoryMB',
      });
    }

    await Optimize.visit();

    assert.equal(Layout.breadcrumbFor('optimize').text, 'Recommendations');

    assert.equal(
      Optimize.recommendationSummaries[0].slug,
      `${this.job1.name} / ${currentTaskGroup.name}`
    );

    assert.equal(
      Layout.breadcrumbFor('optimize.summary').text,
      `${this.job1.name} / ${currentTaskGroup.name}`
    );

    assert.equal(
      Optimize.recommendationSummaries[0].namespace,
      this.job1.namespace
    );

    assert.equal(
      Optimize.recommendationSummaries[1].slug,
      `${this.job2.name} / ${nextTaskGroup.name}`
    );

    const currentRecommendations = currentTaskGroup.tasks.models.reduce(
      (recommendations, task) =>
        recommendations.concat(task.recommendations.models),
      []
    );
    const latestSubmitTime = Math.max(
      ...currentRecommendations.mapBy('submitTime')
    );

    Optimize.recommendationSummaries[0].as((summary) => {
      assert.equal(
        summary.date,
        moment(new Date(latestSubmitTime / 1000000)).format(
          'MMM DD HH:mm:ss ZZ'
        )
      );

      const currentTaskGroupAllocations = server.schema.allocations.where({
        jobId: currentTaskGroup.job.name,
        taskGroup: currentTaskGroup.name,
      });
      assert.equal(summary.allocationCount, currentTaskGroupAllocations.length);

      const { currCpu, currMem } = currentTaskGroup.tasks.models.reduce(
        (currentResources, task) => {
          currentResources.currCpu += task.resources.CPU;
          currentResources.currMem += task.resources.MemoryMB;
          return currentResources;
        },
        { currCpu: 0, currMem: 0 }
      );

      const { recCpu, recMem } = currentRecommendations.reduce(
        (recommendedResources, recommendation) => {
          if (recommendation.resource === 'CPU') {
            recommendedResources.recCpu += recommendation.value;
          } else {
            recommendedResources.recMem += recommendation.value;
          }

          return recommendedResources;
        },
        { recCpu: 0, recMem: 0 }
      );

      const cpuDiff = recCpu > 0 ? recCpu - currCpu : 0;
      const memDiff = recMem > 0 ? recMem - currMem : 0;

      const cpuSign = cpuDiff > 0 ? '+' : '';
      const memSign = memDiff > 0 ? '+' : '';

      const cpuDiffPercent = Math.round((100 * cpuDiff) / currCpu);
      const memDiffPercent = Math.round((100 * memDiff) / currMem);

      assert.equal(
        replaceMinus(summary.cpu),
        cpuDiff
          ? `${cpuSign}${formatHertz(
              cpuDiff,
              'MHz'
            )} ${cpuSign}${cpuDiffPercent}%`
          : ''
      );
      assert.equal(
        replaceMinus(summary.memory),
        memDiff
          ? `${memSign}${formattedMemDiff(
              memDiff
            )} ${memSign}${memDiffPercent}%`
          : ''
      );

      assert.equal(
        replaceMinus(summary.aggregateCpu),
        cpuDiff
          ? `${cpuSign}${formatHertz(
              cpuDiff * currentTaskGroupAllocations.length,
              'MHz'
            )}`
          : ''
      );

      assert.equal(
        replaceMinus(summary.aggregateMemory),
        memDiff
          ? `${memSign}${formattedMemDiff(
              memDiff * currentTaskGroupAllocations.length
            )}`
          : ''
      );
    });

    assert.ok(Optimize.recommendationSummaries[0].isActive);
    assert.notOk(Optimize.recommendationSummaries[1].isActive);

    assert.equal(Optimize.card.slug.jobName, this.job1.name);
    assert.equal(Optimize.card.slug.groupName, currentTaskGroup.name);

    const summaryMemoryBefore = Optimize.recommendationSummaries[0].memory;

    let toggledAnything = true;

    // Toggle off all memory
    if (Optimize.card.togglesTable.toggleAllMemory.isPresent) {
      await Optimize.card.togglesTable.toggleAllMemory.toggle();

      assert.notOk(Optimize.card.togglesTable.tasks[0].memory.isActive);
      assert.notOk(Optimize.card.togglesTable.tasks[1].memory.isActive);
    } else if (!Optimize.card.togglesTable.tasks[0].cpu.isDisabled) {
      await Optimize.card.togglesTable.tasks[0].memory.toggle();
    } else {
      toggledAnything = false;
    }

    assert.equal(
      Optimize.recommendationSummaries[0].memory,
      summaryMemoryBefore,
      'toggling recommendations doesn’t affect the summary table diffs'
    );

    const currentTaskIds = currentTaskGroup.tasks.models.mapBy('id');
    const taskIdFilter = (task) => currentTaskIds.includes(task.taskId);

    const cpuRecommendationIds = server.schema.recommendations
      .where({ resource: 'CPU' })
      .models.filter(taskIdFilter)
      .mapBy('id');

    const memoryRecommendationIds = server.schema.recommendations
      .where({ resource: 'MemoryMB' })
      .models.filter(taskIdFilter)
      .mapBy('id');

    const appliedIds = toggledAnything
      ? cpuRecommendationIds
      : memoryRecommendationIds;
    const dismissedIds = toggledAnything ? memoryRecommendationIds : [];

    await Optimize.card.acceptButton.click();

    const request = server.pretender.handledRequests
      .filterBy('method', 'POST')
      .pop();
    const { Apply, Dismiss } = JSON.parse(request.requestBody);

    assert.equal(request.url, '/v1/recommendations/apply');

    assert.deepEqual(Apply, appliedIds);
    assert.deepEqual(Dismiss, dismissedIds);

    assert.equal(Optimize.card.slug.jobName, this.job2.name);
    assert.equal(Optimize.card.slug.groupName, nextTaskGroup.name);

    assert.ok(Optimize.recommendationSummaries[1].isActive);
  });

  test('can navigate between summaries via the table', async function (assert) {
    server.createList('job', 10, {
      createRecommendations: true,
      groupsCount: 1,
      groupTaskCount: 2,
      namespaceId: server.db.namespaces[1].id,
    });

    await Optimize.visit();
    await Optimize.recommendationSummaries[1].click();

    assert.equal(
      `${Optimize.card.slug.jobName} / ${Optimize.card.slug.groupName}`,
      Optimize.recommendationSummaries[1].slug
    );
    assert.ok(Optimize.recommendationSummaries[1].isActive);
  });

  test('can visit a summary directly via URL', async function (assert) {
    server.createList('job', 10, {
      createRecommendations: true,
      groupsCount: 1,
      groupTaskCount: 2,
      namespaceId: server.db.namespaces[1].id,
    });

    await Optimize.visit();

    const lastSummary =
      Optimize.recommendationSummaries[
        Optimize.recommendationSummaries.length - 1
      ];
    const collapsedSlug = lastSummary.slug.replace(' / ', '/');

    // preferable to use page object’s visitable but it encodes the slash
    await visit(
      `/optimize/${collapsedSlug}?namespace=${lastSummary.namespace}`
    );

    assert.equal(
      `${Optimize.card.slug.jobName} / ${Optimize.card.slug.groupName}`,
      lastSummary.slug
    );
    assert.ok(lastSummary.isActive);
    assert.equal(
      currentURL(),
      `/optimize/${collapsedSlug}?namespace=${lastSummary.namespace}`
    );
  });

  test('when a summary is not found, an error message is shown, but the URL persists', async function (assert) {
    await visit('/optimize/nonexistent/summary?namespace=anamespace');

    assert.equal(
      currentURL(),
      '/optimize/nonexistent/summary?namespace=anamespace'
    );
    assert.ok(Optimize.applicationError.isPresent);
    assert.equal(Optimize.applicationError.title, 'Not Found');
  });

  test('cannot return to already-processed summaries', async function (assert) {
    await Optimize.visit();
    await Optimize.card.acceptButton.click();

    assert.ok(Optimize.recommendationSummaries[0].isDisabled);

    await Optimize.recommendationSummaries[0].click();

    assert.ok(Optimize.recommendationSummaries[1].isActive);
  });

  test('can dismiss a set of recommendations', async function (assert) {
    await Optimize.visit();

    const currentTaskGroup = this.job1.taskGroups.models[0];
    const currentTaskIds = currentTaskGroup.tasks.models.mapBy('id');
    const taskIdFilter = (task) => currentTaskIds.includes(task.taskId);

    const idsBeforeDismissal = server.schema.recommendations
      .all()
      .models.filter(taskIdFilter)
      .mapBy('id');

    await Optimize.card.dismissButton.click();

    const request = server.pretender.handledRequests
      .filterBy('method', 'POST')
      .pop();
    const { Apply, Dismiss } = JSON.parse(request.requestBody);

    assert.equal(request.url, '/v1/recommendations/apply');

    assert.deepEqual(Apply, []);
    assert.deepEqual(Dismiss, idsBeforeDismissal);
  });

  test('it displays an error encountered trying to save and proceeds to the next summary when the error is dismissed', async function (assert) {
    server.post('/recommendations/apply', function () {
      return new Response(500, {}, null);
    });

    await Optimize.visit();
    await Optimize.card.acceptButton.click();

    assert.ok(Optimize.error.isPresent);
    assert.equal(Optimize.error.headline, 'Recommendation error');
    assert.equal(
      Optimize.error.errors,
      'Error: Ember Data Request POST /v1/recommendations/apply returned a 500 Payload (application/json)'
    );

    await Optimize.error.dismiss();
    assert.equal(Optimize.card.slug.jobName, this.job2.name);
  });

  test('it displays an empty message when there are no recommendations', async function (assert) {
    server.db.recommendations.remove();
    await Optimize.visit();

    assert.ok(Optimize.empty.isPresent);
    assert.equal(Optimize.empty.headline, 'No Recommendations');
  });

  test('it displays an empty message after all recommendations have been processed', async function (assert) {
    await Optimize.visit();

    await Optimize.card.acceptButton.click();
    await Optimize.card.acceptButton.click();

    assert.ok(Optimize.empty.isPresent);
  });

  test('it redirects to jobs and hides the gutter link when the token lacks permissions', async function (assert) {
    window.localStorage.nomadTokenSecret = clientToken.secretId;
    await Optimize.visit();

    assert.equal(currentURL(), '/jobs?namespace=*');
    assert.ok(Layout.gutter.optimize.isHidden);
  });

  test('it reloads partially-loaded jobs', async function (assert) {
    await JobsList.visit();
    await Optimize.visit();

    assert.equal(Optimize.recommendationSummaries.length, 2);
  });
});

module('Acceptance | optimize search and facets', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    server.create('feature', { name: 'Dynamic Application Sizing' });

    server.create('node-pool');
    server.create('node');

    server.createList('namespace', 2);

    managementToken = server.create('token');

    window.localStorage.clear();
    window.localStorage.nomadTokenSecret = managementToken.secretId;
  });

  test('search field narrows summary table results, changes the active summary if it no longer matches, and displays a no matches message when there are none', async function (assert) {
    server.create('job', {
      name: 'zzzzzz',
      createRecommendations: true,
      groupsCount: 1,
      groupTaskCount: 6,
    });

    // Ensure this job’s recommendations are sorted to the top of the table
    const futureSubmitTime = (Date.now() + 10000) * 1000000;
    server.db.recommendations.update({ submitTime: futureSubmitTime });

    server.create('job', {
      name: 'oooooo',
      createRecommendations: true,
      groupsCount: 2,
      groupTaskCount: 4,
    });

    server.create('job', {
      name: 'pppppp',
      createRecommendations: true,
      groupsCount: 2,
      groupTaskCount: 4,
    });

    await Optimize.visit();

    assert.equal(Optimize.card.slug.jobName, 'zzzzzz');

    assert.equal(
      collapseWhitespace(Optimize.search.placeholder),
      `Search ${Optimize.recommendationSummaries.length} recommendations...`
    );

    await Optimize.search.fillIn('ooo');

    assert.equal(Optimize.recommendationSummaries.length, 2);
    assert.ok(Optimize.recommendationSummaries[0].slug.startsWith('oooooo'));

    assert.equal(Optimize.card.slug.jobName, 'oooooo');
    assert.ok(currentURL().includes('oooooo'));

    await Optimize.search.fillIn('qqq');

    assert.notOk(Optimize.card.isPresent);
    assert.ok(Optimize.empty.isPresent);
    assert.equal(Optimize.empty.headline, 'No Matches');
    assert.equal(currentURL(), '/optimize?search=qqq');

    await Optimize.search.fillIn('');

    assert.equal(Optimize.card.slug.jobName, 'zzzzzz');
    assert.ok(Optimize.recommendationSummaries[0].isActive);
  });

  test('the namespaces toggle doesn’t show when there aren’t namespaces', async function (assert) {
    server.db.namespaces.remove();

    server.create('job', {
      createRecommendations: true,
      groupsCount: 1,
      groupTaskCount: 4,
    });

    await Optimize.visit();

    assert.ok(Optimize.facets.namespace.isHidden);
  });

  test('processing a summary moves to the next one in the sorted list', async function (assert) {
    server.create('job', {
      name: 'ooo111',
      createRecommendations: true,
      groupsCount: 1,
      groupTaskCount: 4,
    });

    server.create('job', {
      name: 'pppppp',
      createRecommendations: true,
      groupsCount: 1,
      groupTaskCount: 4,
    });

    server.create('job', {
      name: 'ooo222',
      createRecommendations: true,
      groupsCount: 1,
      groupTaskCount: 4,
    });

    // Directly set the sorting of the above jobs’s summaries in the table
    const futureSubmitTime = (Date.now() + 10000) * 1000000;
    const nowSubmitTime = Date.now() * 1000000;
    const pastSubmitTime = (Date.now() - 10000) * 1000000;

    const jobNameToRecommendationSubmitTime = {
      ooo111: futureSubmitTime,
      pppppp: nowSubmitTime,
      ooo222: pastSubmitTime,
    };

    server.schema.recommendations.all().models.forEach((recommendation) => {
      const parentJob = recommendation.task.taskGroup.job;
      const submitTimeForJob =
        jobNameToRecommendationSubmitTime[parentJob.name];
      recommendation.submitTime = submitTimeForJob;
      recommendation.save();
    });

    await Optimize.visit();
    await Optimize.search.fillIn('ooo');
    await Optimize.card.acceptButton.click();

    assert.equal(Optimize.card.slug.jobName, 'ooo222');
  });

  test('the optimize page has appropriate faceted search options', async function (assert) {
    server.createList('job', 4, {
      status: 'running',
      createRecommendations: true,
      childrenCount: 0,
    });

    await Optimize.visit();

    assert.ok(Optimize.facets.namespace.isPresent, 'Namespace facet found');
    assert.ok(Optimize.facets.type.isPresent, 'Type facet found');
    assert.ok(Optimize.facets.status.isPresent, 'Status facet found');
    assert.ok(Optimize.facets.datacenter.isPresent, 'Datacenter facet found');
    assert.ok(Optimize.facets.prefix.isPresent, 'Prefix facet found');
  });

  testSingleSelectFacet('Namespace', {
    facet: Optimize.facets.namespace,
    paramName: 'namespace',
    expectedOptions: ['All (*)', 'default', 'namespace-1'],
    optionToSelect: 'namespace-1',
    async beforeEach() {
      server.createList('job', 2, {
        namespaceId: 'default',
        createRecommendations: true,
      });
      server.createList('job', 2, {
        namespaceId: 'namespace-1',
        createRecommendations: true,
      });
      await Optimize.visit();
    },
    filter(taskGroup, selection) {
      return taskGroup.job.namespaceId === selection;
    },
  });

  testFacet('Type', {
    facet: Optimize.facets.type,
    paramName: 'type',
    expectedOptions: ['Service', 'System'],
    async beforeEach() {
      server.createList('job', 2, {
        type: 'service',
        createRecommendations: true,
        groupsCount: 1,
        groupTaskCount: 2,
      });

      server.createList('job', 2, {
        type: 'system',
        createRecommendations: true,
        groupsCount: 1,
        groupTaskCount: 2,
      });
      await Optimize.visit();
    },
    filter(taskGroup, selection) {
      let displayType = taskGroup.job.type;
      return selection.includes(displayType);
    },
  });

  testFacet('Status', {
    facet: Optimize.facets.status,
    paramName: 'status',
    expectedOptions: ['Pending', 'Running', 'Dead'],
    async beforeEach() {
      server.createList('job', 2, {
        status: 'pending',
        createRecommendations: true,
        groupsCount: 1,
        groupTaskCount: 2,
        childrenCount: 0,
      });
      server.createList('job', 2, {
        status: 'running',
        createRecommendations: true,
        groupsCount: 1,
        groupTaskCount: 2,
        childrenCount: 0,
      });
      server.createList('job', 2, {
        status: 'dead',
        createRecommendations: true,
        childrenCount: 0,
      });
      await Optimize.visit();
    },
    filter: (taskGroup, selection) => selection.includes(taskGroup.job.status),
  });

  testFacet('Datacenter', {
    facet: Optimize.facets.datacenter,
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
        createRecommendations: true,
        groupsCount: 1,
        groupTaskCount: 2,
        childrenCount: 0,
      });
      server.create('job', {
        datacenters: ['pdx', 'ord'],
        createRecommendations: true,
        groupsCount: 1,
        groupTaskCount: 2,
        childrenCount: 0,
      });
      server.create('job', {
        datacenters: ['lax', 'jfk'],
        createRecommendations: true,
        groupsCount: 1,
        groupTaskCount: 2,
        childrenCount: 0,
      });
      server.create('job', {
        datacenters: ['jfk', 'dfw'],
        createRecommendations: true,
        groupsCount: 1,
        groupTaskCount: 2,
        childrenCount: 0,
      });
      server.create('job', {
        datacenters: ['pdx'],
        createRecommendations: true,
        childrenCount: 0,
      });
      await Optimize.visit();
    },
    filter: (taskGroup, selection) =>
      taskGroup.job.datacenters.find((dc) => selection.includes(dc)),
  });

  testFacet('Prefix', {
    facet: Optimize.facets.prefix,
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
      ].forEach((name) => {
        server.create('job', {
          name,
          createRecommendations: true,
          createAllocations: true,
          groupsCount: 1,
          groupTaskCount: 2,
          childrenCount: 0,
        });
      });
      await Optimize.visit();
    },
    filter: (taskGroup, selection) =>
      selection.find((prefix) => taskGroup.job.name.startsWith(prefix)),
  });

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

  function testSingleSelectFacet(
    label,
    { facet, paramName, beforeEach, filter, expectedOptions, optionToSelect }
  ) {
    test(`the ${label} facet has the correct options`, async function (assert) {
      await facetOptions.call(this, assert, beforeEach, facet, expectedOptions);
    });

    test(`the ${label} facet filters the jobs list by ${label}`, async function (assert) {
      await beforeEach();
      await facet.toggle();

      const option = facet.options.findOneBy('label', optionToSelect);
      const selection = option.key;
      await option.select();

      const sortedRecommendations = server.db.recommendations
        .sortBy('submitTime')
        .reverse();

      const recommendationTaskGroups = server.schema.tasks
        .find(sortedRecommendations.mapBy('taskId').uniq())
        .models.mapBy('taskGroup')
        .uniqBy('id')
        .filter((group) => filter(group, selection));

      Optimize.recommendationSummaries.forEach((summary, index) => {
        const group = recommendationTaskGroups[index];
        assert.equal(summary.slug, `${group.job.name} / ${group.name}`);
      });
    });

    test(`selecting an option in the ${label} facet updates the ${paramName} query param`, async function (assert) {
      await beforeEach();
      await facet.toggle();

      const option = facet.options.objectAt(1);
      const selection = option.key;
      await option.select();

      assert.ok(
        currentURL().includes(`${paramName}=${selection}`),
        'URL has the correct query param key and value'
      );
    });
  }

  function testFacet(
    label,
    { facet, paramName, beforeEach, filter, expectedOptions }
  ) {
    test(`the ${label} facet has the correct options`, async function (assert) {
      await facetOptions.call(this, assert, beforeEach, facet, expectedOptions);
    });

    test(`the ${label} facet filters the recommendation summaries by ${label}`, async function (assert) {
      let option;

      await beforeEach();
      await facet.toggle();

      option = facet.options.objectAt(0);
      await option.toggle();

      const selection = [option.key];

      const sortedRecommendations = server.db.recommendations
        .sortBy('submitTime')
        .reverse();

      const recommendationTaskGroups = server.schema.tasks
        .find(sortedRecommendations.mapBy('taskId').uniq())
        .models.mapBy('taskGroup')
        .uniqBy('id')
        .filter((group) => filter(group, selection));

      Optimize.recommendationSummaries.forEach((summary, index) => {
        const group = recommendationTaskGroups[index];
        assert.equal(summary.slug, `${group.job.name} / ${group.name}`);
      });
    });

    test(`selecting multiple options in the ${label} facet results in a broader search`, async function (assert) {
      const selection = [];

      await beforeEach();
      await facet.toggle();

      const option1 = facet.options.objectAt(0);
      const option2 = facet.options.objectAt(1);
      await option1.toggle();
      selection.push(option1.key);
      await option2.toggle();
      selection.push(option2.key);

      const sortedRecommendations = server.db.recommendations
        .sortBy('submitTime')
        .reverse();

      const recommendationTaskGroups = server.schema.tasks
        .find(sortedRecommendations.mapBy('taskId').uniq())
        .models.mapBy('taskGroup')
        .uniqBy('id')
        .filter((group) => filter(group, selection));

      Optimize.recommendationSummaries.forEach((summary, index) => {
        const group = recommendationTaskGroups[index];
        assert.equal(summary.slug, `${group.job.name} / ${group.name}`);
      });
    });

    test(`selecting options in the ${label} facet updates the ${paramName} query param`, async function (assert) {
      const selection = [];

      await beforeEach();
      await facet.toggle();

      const option1 = facet.options.objectAt(0);
      const option2 = facet.options.objectAt(1);
      await option1.toggle();
      selection.push(option1.key);
      await option2.toggle();
      selection.push(option2.key);

      assert.ok(
        currentURL().includes(encodeURIComponent(JSON.stringify(selection)))
      );
    });
  }
});

function formattedMemDiff(memDiff) {
  const absMemDiff = Math.abs(memDiff);
  const negativeSign = memDiff < 0 ? '-' : '';

  return negativeSign + formatBytes(absMemDiff, 'MiB');
}
