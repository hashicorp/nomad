import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { currentURL } from '@ember/test-helpers';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Response from 'ember-cli-mirage/response';
import moment from 'moment';

import Optimize from 'nomad-ui/tests/pages/optimize';
import PageLayout from 'nomad-ui/tests/pages/layout';
import JobsList from 'nomad-ui/tests/pages/jobs/list';

let managementToken, clientToken;

function getLatestRecommendationSubmitTimeForJob(job) {
  const tasks = job.taskGroups.models
    .mapBy('tasks.models')
    .reduce((tasks, taskModels) => tasks.concat(taskModels), []);
  const recommendations = tasks.reduce(
    (recommendations, task) => recommendations.concat(task.recommendations.models),
    []
  );
  return Math.max(...recommendations.mapBy('submitTime'));
}

module('Acceptance | optimize', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function() {
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

  test('it passes an accessibility audit', async function(assert) {
    await Optimize.visit();
    await a11yAudit(assert);
  });

  test('lets recommendations be toggled, reports the choices to the recommendations API, and displays task group recommendations serially', async function(assert) {
    await Optimize.visit();

    const currentTaskGroup = this.job1.taskGroups.models[0];
    const nextTaskGroup = this.job2.taskGroups.models[0];

    assert.equal(Optimize.breadcrumbFor('optimize').text, 'Recommendations');

    assert.equal(
      Optimize.recommendationSummaries[0].slug,
      `${this.job1.name} / ${currentTaskGroup.name}`
    );

    assert.equal(Optimize.recommendationSummaries[0].namespace, this.job1.namespace);

    assert.equal(
      Optimize.recommendationSummaries[1].slug,
      `${this.job2.name} / ${nextTaskGroup.name}`
    );

    const currentRecommendations = currentTaskGroup.tasks.models.reduce(
      (recommendations, task) => recommendations.concat(task.recommendations.models),
      []
    );
    const latestSubmitTime = Math.max(...currentRecommendations.mapBy('submitTime'));

    Optimize.recommendationSummaries[0].as(summary => {
      assert.equal(
        summary.date,
        moment(new Date(latestSubmitTime / 1000000)).format('MMM DD HH:mm:ss ZZ')
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
        summary.cpu,
        cpuDiff ? `${cpuSign}${cpuDiff} MHz ${cpuSign}${cpuDiffPercent}%` : ''
      );
      assert.equal(
        summary.memory,
        memDiff ? `${memSign}${formattedMemDiff(memDiff)} ${memSign}${memDiffPercent}%` : ''
      );

      assert.equal(
        summary.aggregateCpu,
        cpuDiff ? `${cpuSign}${cpuDiff * currentTaskGroupAllocations.length} MHz` : ''
      );

      assert.equal(
        summary.aggregateMemory,
        memDiff ? `${memSign}${formattedMemDiff(memDiff * currentTaskGroupAllocations.length)}` : ''
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
    const taskIdFilter = task => currentTaskIds.includes(task.taskId);

    const cpuRecommendationIds = server.schema.recommendations
      .where({ resource: 'CPU' })
      .models.filter(taskIdFilter)
      .mapBy('id');

    const memoryRecommendationIds = server.schema.recommendations
      .where({ resource: 'MemoryMB' })
      .models.filter(taskIdFilter)
      .mapBy('id');

    const appliedIds = toggledAnything ? cpuRecommendationIds : memoryRecommendationIds;
    const dismissedIds = toggledAnything ? memoryRecommendationIds : [];

    await Optimize.card.acceptButton.click();

    const request = server.pretender.handledRequests.filterBy('method', 'POST').pop();
    const { Apply, Dismiss } = JSON.parse(request.requestBody);

    assert.equal(request.url, '/v1/recommendations/apply');

    assert.deepEqual(Apply, appliedIds);
    assert.deepEqual(Dismiss, dismissedIds);

    assert.equal(Optimize.card.slug.jobName, this.job2.name);
    assert.equal(Optimize.card.slug.groupName, nextTaskGroup.name);

    assert.ok(Optimize.recommendationSummaries[1].isActive);
  });

  test('can navigate between summaries via the table', async function(assert) {
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

  test('cannot return to already-processed summaries', async function(assert) {
    await Optimize.visit();
    await Optimize.card.acceptButton.click();

    assert.ok(Optimize.recommendationSummaries[0].isDisabled);

    await Optimize.recommendationSummaries[0].click();

    assert.ok(Optimize.recommendationSummaries[1].isActive);
  });

  test('can dismiss a set of recommendations', async function(assert) {
    await Optimize.visit();

    const currentTaskGroup = this.job1.taskGroups.models[0];
    const currentTaskIds = currentTaskGroup.tasks.models.mapBy('id');
    const taskIdFilter = task => currentTaskIds.includes(task.taskId);

    const idsBeforeDismissal = server.schema.recommendations
      .all()
      .models.filter(taskIdFilter)
      .mapBy('id');

    await Optimize.card.dismissButton.click();

    const request = server.pretender.handledRequests.filterBy('method', 'POST').pop();
    const { Apply, Dismiss } = JSON.parse(request.requestBody);

    assert.equal(request.url, '/v1/recommendations/apply');

    assert.deepEqual(Apply, []);
    assert.deepEqual(Dismiss, idsBeforeDismissal);
  });

  test('it displays an error encountered trying to save and proceeds to the next summary when the error is dismiss', async function(assert) {
    server.post('/recommendations/apply', function() {
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

  test('it displays an empty message when there are no recommendations', async function(assert) {
    server.db.recommendations.remove();
    await Optimize.visit();

    assert.ok(Optimize.empty.isPresent);
    assert.equal(Optimize.empty.headline, 'No Recommendations');
  });

  test('it displays an empty message after all recommendations have been processed', async function(assert) {
    await Optimize.visit();

    await Optimize.card.acceptButton.click();
    await Optimize.card.acceptButton.click();

    assert.ok(Optimize.empty.isPresent);
  });

  test('it redirects to jobs and hides the gutter link when the token lacks permissions', async function(assert) {
    window.localStorage.nomadTokenSecret = clientToken.secretId;
    await Optimize.visit();

    assert.equal(currentURL(), '/jobs');
    assert.ok(PageLayout.gutter.optimize.isHidden);
  });

  test('it reloads partially-loaded jobs', async function(assert) {
    await JobsList.visit();
    await Optimize.visit();

    assert.equal(Optimize.recommendationSummaries.length, 2);
  });
});

function formattedMemDiff(memDiff) {
  const absMemDiff = Math.abs(memDiff);
  const negativeSign = memDiff < 0 ? '-' : '';

  if (absMemDiff >= 1024) {
    const gibDiff = absMemDiff / 1024;

    if (Number.isInteger(gibDiff)) {
      return `${negativeSign}${gibDiff} GiB`;
    } else {
      return `${negativeSign}${gibDiff.toFixed(2)} GiB`;
    }
  } else {
    return `${negativeSign}${absMemDiff} MiB`;
  }
}
