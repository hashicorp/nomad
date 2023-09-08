/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, settled } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';
import Service from '@ember/service';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

import RecommendationCardComponent from 'nomad-ui/tests/pages/components/recommendation-card';
import { create } from 'ember-cli-page-object';
const RecommendationCard = create(RecommendationCardComponent);

import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { set } from '@ember/object';

module('Integration | Component | das/recommendation-card', function (hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function () {
    const mockRouter = Service.extend({
      init() {
        this._super(...arguments);
      },

      urlFor(route, slug, { queryParams: { namespace } }) {
        return `${route}:${slug}?namespace=${namespace}`;
      },
    });

    this.owner.register('service:router', mockRouter);
  });

  test('it renders a recommendation card', async function (assert) {
    assert.expect(49);

    const task1 = {
      name: 'jortle',
      reservedCPU: 150,
      reservedMemory: 128,
    };

    const task2 = {
      name: 'tortle',
      reservedCPU: 125,
      reservedMemory: 256,
    };

    this.set(
      'summary',
      new MockRecommendationSummary({
        jobNamespace: 'namespace',
        recommendations: [
          {
            resource: 'MemoryMB',
            stats: {},
            task: task1,
            value: 192,
            currentValue: task1.reservedMemory,
          },
          {
            resource: 'CPU',
            stats: {},
            task: task1,
            value: 50,
            currentValue: task1.reservedCPU,
          },
          {
            resource: 'CPU',
            stats: {},
            task: task2,
            value: 150,
            currentValue: task2.reservedCPU,
          },
          {
            resource: 'MemoryMB',
            stats: {},
            task: task2,
            value: 320,
            currentValue: task2.reservedMemory,
          },
        ],

        taskGroup: {
          count: 2,
          name: 'group-name',
          job: {
            name: 'job-name',
            namespace: {
              name: 'namespace',
            },
          },
          reservedCPU: task1.reservedCPU + task2.reservedCPU,
          reservedMemory: task1.reservedMemory + task2.reservedMemory,
        },
      })
    );

    await render(hbs`<Das::RecommendationCard @summary={{this.summary}} />`);

    assert.equal(RecommendationCard.slug.jobName, 'job-name');
    assert.equal(RecommendationCard.slug.groupName, 'group-name');

    assert.equal(RecommendationCard.namespace, 'namespace');

    assert.equal(RecommendationCard.totalsTable.current.cpu.text, '275 MHz');
    assert.equal(RecommendationCard.totalsTable.current.memory.text, '384 MiB');

    RecommendationCard.totalsTable.recommended.cpu.as((RecommendedCpu) => {
      assert.equal(RecommendedCpu.text, '200 MHz');
      assert.ok(RecommendedCpu.isDecrease);
    });

    RecommendationCard.totalsTable.recommended.memory.as(
      (RecommendedMemory) => {
        assert.equal(RecommendedMemory.text, '512 MiB');
        assert.ok(RecommendedMemory.isIncrease);
      }
    );

    assert.equal(RecommendationCard.totalsTable.unitDiff.cpu, '-75 MHz');
    assert.equal(RecommendationCard.totalsTable.unitDiff.memory, '+128 MiB');

    // Expected signal has a minus character, not a hyphen.
    assert.equal(RecommendationCard.totalsTable.percentDiff.cpu, '−27%');
    assert.equal(RecommendationCard.totalsTable.percentDiff.memory, '+33%');

    assert.dom('.copy-button').hasTextContaining('job-name / group-name');

    const clipboardText = document
      .querySelector('.copy-button > button')
      .getAttribute('data-clipboard-text');
    assert.ok(
      clipboardText.endsWith(
        'optimize.summary:job-name/group-name?namespace=namespace'
      )
    );

    assert.equal(
      RecommendationCard.activeTask.totalsTable.current.cpu.text,
      '150 MHz'
    );
    assert.equal(
      RecommendationCard.activeTask.totalsTable.current.memory.text,
      '128 MiB'
    );

    RecommendationCard.activeTask.totalsTable.recommended.cpu.as(
      (RecommendedCpu) => {
        assert.equal(RecommendedCpu.text, '50 MHz');
        assert.ok(RecommendedCpu.isDecrease);
      }
    );

    RecommendationCard.activeTask.totalsTable.recommended.memory.as(
      (RecommendedMemory) => {
        assert.equal(RecommendedMemory.text, '192 MiB');
        assert.ok(RecommendedMemory.isIncrease);
      }
    );

    assert.equal(RecommendationCard.activeTask.charts.length, 2);
    assert.equal(
      RecommendationCard.activeTask.charts[0].resource,
      'CPU',
      'CPU chart should be first when present'
    );

    assert.ok(RecommendationCard.activeTask.cpuChart.isDecrease);
    assert.ok(RecommendationCard.activeTask.memoryChart.isIncrease);

    assert.equal(RecommendationCard.togglesTable.tasks.length, 2);

    await RecommendationCard.togglesTable.tasks[0].as(async (FirstTask) => {
      assert.equal(FirstTask.name, 'jortle');
      assert.ok(FirstTask.isActive);

      assert.equal(FirstTask.cpu.title, 'CPU for jortle');
      assert.ok(FirstTask.cpu.isActive);

      assert.equal(FirstTask.memory.title, 'Memory for jortle');
      assert.ok(FirstTask.memory.isActive);

      await FirstTask.cpu.toggle();

      assert.notOk(FirstTask.cpu.isActive);
      assert.ok(RecommendationCard.activeTask.cpuChart.isDisabled);
    });

    assert.notOk(RecommendationCard.togglesTable.tasks[1].isActive);

    assert.equal(RecommendationCard.activeTask.name, 'jortle task');

    RecommendationCard.totalsTable.recommended.cpu.as((RecommendedCpu) => {
      assert.equal(RecommendedCpu.text, '300 MHz');
      assert.ok(RecommendedCpu.isIncrease);
    });

    RecommendationCard.activeTask.totalsTable.recommended.cpu.as(
      (RecommendedCpu) => {
        assert.equal(RecommendedCpu.text, '150 MHz');
        assert.ok(RecommendedCpu.isNeutral);
      }
    );

    await RecommendationCard.togglesTable.toggleAllMemory.toggle();

    assert.notOk(RecommendationCard.togglesTable.tasks[0].memory.isActive);
    assert.notOk(RecommendationCard.togglesTable.tasks[1].memory.isActive);

    RecommendationCard.totalsTable.recommended.memory.as(
      (RecommendedMemory) => {
        assert.equal(RecommendedMemory.text, '384 MiB');
        assert.ok(RecommendedMemory.isNeutral);
      }
    );

    await RecommendationCard.togglesTable.tasks[1].click();

    assert.notOk(RecommendationCard.togglesTable.tasks[0].isActive);
    assert.ok(RecommendationCard.togglesTable.tasks[1].isActive);

    assert.equal(RecommendationCard.activeTask.name, 'tortle task');
    assert.equal(
      RecommendationCard.activeTask.totalsTable.current.cpu.text,
      '125 MHz'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('it doesn’t have header toggles when there’s only one task', async function (assert) {
    const task1 = {
      name: 'jortle',
      reservedCPU: 150,
      reservedMemory: 128,
    };

    this.set(
      'summary',
      new MockRecommendationSummary({
        recommendations: [
          {
            resource: 'CPU',
            stats: {},
            task: task1,
            value: 50,
          },
          {
            resource: 'MemoryMB',
            stats: {},
            task: task1,
            value: 192,
          },
        ],

        taskGroup: {
          count: 1,
          reservedCPU: task1.reservedCPU,
          reservedMemory: task1.reservedMemory,
        },
      })
    );

    await render(hbs`<Das::RecommendationCard @summary={{this.summary}} />`);

    assert.notOk(RecommendationCard.togglesTable.toggleAllIsPresent);
    assert.notOk(RecommendationCard.togglesTable.toggleAllCPU.isPresent);
    assert.notOk(RecommendationCard.togglesTable.toggleAllMemory.isPresent);
  });

  test('it disables the accept button when all recommendations are disabled', async function (assert) {
    const task1 = {
      name: 'jortle',
      reservedCPU: 150,
      reservedMemory: 128,
    };

    this.set(
      'summary',
      new MockRecommendationSummary({
        recommendations: [
          {
            resource: 'CPU',
            stats: {},
            task: task1,
            value: 50,
          },
          {
            resource: 'MemoryMB',
            stats: {},
            task: task1,
            value: 192,
          },
        ],

        taskGroup: {
          count: 1,
          reservedCPU: task1.reservedCPU,
          reservedMemory: task1.reservedMemory,
        },
      })
    );

    await render(hbs`<Das::RecommendationCard @summary={{this.summary}} />`);

    await RecommendationCard.togglesTable.tasks[0].cpu.toggle();
    await RecommendationCard.togglesTable.tasks[0].memory.toggle();

    assert.ok(RecommendationCard.acceptButton.isDisabled);
  });

  test('it doesn’t show a toggle or chart when there’s no recommendation for that resource', async function (assert) {
    const task1 = {
      name: 'jortle',
      reservedCPU: 150,
      reservedMemory: 128,
    };

    this.set(
      'summary',
      new MockRecommendationSummary({
        recommendations: [
          {
            resource: 'CPU',
            stats: {},
            task: task1,
            value: 50,
          },
        ],

        taskGroup: {
          count: 2,
          name: 'group-name',
          job: {
            name: 'job-name',
          },
          reservedCPU: task1.reservedCPU,
          reservedMemory: task1.reservedMemory,
        },
      })
    );

    await render(hbs`<Das::RecommendationCard @summary={{this.summary}} />`);

    assert.equal(
      RecommendationCard.totalsTable.recommended.memory.text,
      '128 MiB'
    );
    assert.equal(RecommendationCard.totalsTable.unitDiff.memory, '0 MiB');
    assert.equal(RecommendationCard.totalsTable.percentDiff.memory, '+0%');

    assert.equal(
      RecommendationCard.narrative.trim(),
      'Applying the selected recommendations will save an aggregate 200 MHz of CPU across 2 allocations.'
    );

    assert.ok(RecommendationCard.togglesTable.tasks[0].memory.isDisabled);
    assert.notOk(RecommendationCard.activeTask.memoryChart.isPresent);
  });

  test('it disables a resource’s toggle all toggle when there are no recommendations for it', async function (assert) {
    const task1 = {
      name: 'jortle',
      reservedCPU: 150,
      reservedMemory: 128,
    };

    const task2 = {
      name: 'tortle',
      reservedCPU: 150,
      reservedMemory: 128,
    };

    this.set(
      'summary',
      new MockRecommendationSummary({
        recommendations: [
          {
            resource: 'CPU',
            stats: {},
            task: task1,
            value: 50,
          },
          {
            resource: 'CPU',
            stats: {},
            task: task2,
            value: 50,
          },
        ],

        taskGroup: {
          count: 2,
          name: 'group-name',
          job: {
            name: 'job-name',
          },
          reservedCPU: task1.reservedCPU + task2.reservedCPU,
          reservedMemory: task1.reservedMemory + task2.reservedMemory,
        },
      })
    );

    await render(hbs`<Das::RecommendationCard @summary={{this.summary}} />`);

    assert.ok(RecommendationCard.togglesTable.toggleAllMemory.isDisabled);
    assert.notOk(RecommendationCard.togglesTable.toggleAllMemory.isActive);
    assert.notOk(RecommendationCard.activeTask.memoryChart.isPresent);
  });

  test('it renders diff calculations in a sentence', async function (assert) {
    const task1 = {
      name: 'jortle',
      reservedCPU: 150,
      reservedMemory: 128,
    };

    const task2 = {
      name: 'tortle',
      reservedCPU: 125,
      reservedMemory: 256,
    };

    this.set(
      'summary',
      new MockRecommendationSummary({
        recommendations: [
          {
            resource: 'CPU',
            stats: {},
            task: task1,
            value: 50,
            currentValue: task1.reservedCPU,
          },
          {
            resource: 'MemoryMB',
            stats: {},
            task: task1,
            value: 192,
            currentValue: task1.reservedMemory,
          },
          {
            resource: 'CPU',
            stats: {},
            task: task2,
            value: 150,
            currentValue: task2.reservedCPU,
          },
          {
            resource: 'MemoryMB',
            stats: {},
            task: task2,
            value: 320,
            currentValue: task2.reservedMemory,
          },
        ],

        taskGroup: {
          count: 10,
          name: 'group-name',
          job: {
            name: 'job-name',
            namespace: {
              name: 'namespace',
            },
          },
          reservedCPU: task1.reservedCPU + task2.reservedCPU,
          reservedMemory: task1.reservedMemory + task2.reservedMemory,
        },
      })
    );

    await render(hbs`<Das::RecommendationCard @summary={{this.summary}} />`);

    const [cpuRec1, memRec1, cpuRec2, memRec2] = this.summary.recommendations;

    assert.equal(
      RecommendationCard.narrative.trim(),
      'Applying the selected recommendations will save an aggregate 750 MHz of CPU and add an aggregate 1.25 GiB of memory across 10 allocations.'
    );

    this.summary.toggleRecommendation(cpuRec1);
    await settled();

    assert.equal(
      RecommendationCard.narrative.trim(),
      'Applying the selected recommendations will add an aggregate 250 MHz of CPU and 1.25 GiB of memory across 10 allocations.'
    );

    this.summary.toggleRecommendation(memRec1);
    await settled();

    assert.equal(
      RecommendationCard.narrative.trim(),
      'Applying the selected recommendations will add an aggregate 250 MHz of CPU and 640 MiB of memory across 10 allocations.'
    );

    this.summary.toggleRecommendation(cpuRec2);
    await settled();

    assert.equal(
      RecommendationCard.narrative.trim(),
      'Applying the selected recommendations will add an aggregate 640 MiB of memory across 10 allocations.'
    );

    this.summary.toggleRecommendation(cpuRec1);
    this.summary.toggleRecommendation(memRec2);
    await settled();

    assert.equal(
      RecommendationCard.narrative.trim(),
      'Applying the selected recommendations will save an aggregate 1 GHz of CPU across 10 allocations.'
    );

    this.summary.toggleRecommendation(cpuRec1);
    await settled();

    assert.equal(RecommendationCard.narrative.trim(), '');

    this.summary.toggleRecommendation(cpuRec1);
    await settled();

    assert.equal(
      RecommendationCard.narrative.trim(),
      'Applying the selected recommendations will save an aggregate 1 GHz of CPU across 10 allocations.'
    );

    this.summary.toggleRecommendation(memRec2);
    set(memRec2, 'value', 128);
    await settled();

    assert.equal(
      RecommendationCard.narrative.trim(),
      'Applying the selected recommendations will save an aggregate 1 GHz of CPU and 1.25 GiB of memory across 10 allocations.'
    );
  });

  test('it renders diff calculations in a sentence with no aggregation for one allocatio', async function (assert) {
    const task1 = {
      name: 'jortle',
      reservedCPU: 150,
      reservedMemory: 128,
    };

    const task2 = {
      name: 'tortle',
      reservedCPU: 125,
      reservedMemory: 256,
    };

    this.set(
      'summary',
      new MockRecommendationSummary({
        recommendations: [
          {
            resource: 'CPU',
            stats: {},
            task: task1,
            value: 50,
            currentValue: task1.reservedCPU,
          },
          {
            resource: 'MemoryMB',
            stats: {},
            task: task1,
            value: 192,
            currentValue: task1.reservedMemory,
          },
          {
            resource: 'CPU',
            stats: {},
            task: task2,
            value: 150,
            currentValue: task2.reservedCPU,
          },
          {
            resource: 'MemoryMB',
            stats: {},
            task: task2,
            value: 320,
            currentValue: task2.reservedMemory,
          },
        ],

        taskGroup: {
          count: 1,
          name: 'group-name',
          job: {
            name: 'job-name',
            namespace: {
              name: 'namespace',
            },
          },
          reservedCPU: task1.reservedCPU + task2.reservedCPU,
          reservedMemory: task1.reservedMemory + task2.reservedMemory,
        },
      })
    );

    await render(hbs`<Das::RecommendationCard @summary={{this.summary}} />`);

    assert.equal(
      RecommendationCard.narrative.trim(),
      'Applying the selected recommendations will save 75 MHz of CPU and add 128 MiB of memory.'
    );
  });
});

class MockRecommendationSummary {
  @tracked excludedRecommendations = [];

  constructor(attributes) {
    Object.assign(this, attributes);
  }

  get slug() {
    return `${this.taskGroup?.job?.name}/${this.taskGroup?.name}`;
  }

  @action
  toggleRecommendation(recommendation) {
    if (this.excludedRecommendations.includes(recommendation)) {
      this.excludedRecommendations.removeObject(recommendation);
    } else {
      this.excludedRecommendations.pushObject(recommendation);
    }
  }

  @action
  toggleAllRecommendationsForResource(resource, enabled) {
    if (enabled) {
      this.excludedRecommendations = this.excludedRecommendations.rejectBy(
        'resource',
        resource
      );
    } else {
      this.excludedRecommendations.pushObjects(
        this.recommendations.filterBy('resource', resource)
      );
    }
  }
}
