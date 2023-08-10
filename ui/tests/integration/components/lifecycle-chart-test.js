/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/no-conditional-assertions */
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, settled } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { set } from '@ember/object';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import { create } from 'ember-cli-page-object';
import LifecycleChart from 'nomad-ui/tests/pages/components/lifecycle-chart';

const Chart = create(LifecycleChart);

const tasks = [
  {
    lifecycleName: 'main',
    name: 'main two: 3',
  },
  {
    lifecycleName: 'main',
    name: 'main one: 2',
  },
  {
    lifecycleName: 'prestart-ephemeral',
    name: 'prestart ephemeral: 0',
  },
  {
    lifecycleName: 'prestart-sidecar',
    name: 'prestart sidecar: 1',
  },
  {
    lifecycleName: 'poststart-ephemeral',
    name: 'poststart ephemeral: 5',
  },
  {
    lifecycleName: 'poststart-sidecar',
    name: 'poststart sidecar: 4',
  },
  {
    lifecycleName: 'poststop',
    name: 'poststop: 6',
  },
];

module('Integration | Component | lifecycle-chart', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders stateless phases and lifecycle- and name-sorted tasks', async function (assert) {
    assert.expect(32);

    this.set('tasks', tasks);

    await render(hbs`<LifecycleChart @tasks={{tasks}} />`);
    assert.ok(Chart.isPresent);

    assert.equal(Chart.phases[0].name, 'Prestart');
    assert.equal(Chart.phases[1].name, 'Main');
    assert.equal(Chart.phases[2].name, 'Poststart');
    assert.equal(Chart.phases[3].name, 'Poststop');

    Chart.phases.forEach((phase) => assert.notOk(phase.isActive));

    assert.deepEqual(Chart.tasks.mapBy('name'), [
      'prestart ephemeral: 0',
      'prestart sidecar: 1',
      'main one: 2',
      'main two: 3',
      'poststart sidecar: 4',
      'poststart ephemeral: 5',
      'poststop: 6',
    ]);
    assert.deepEqual(Chart.tasks.mapBy('lifecycle'), [
      'Prestart Task',
      'Sidecar Task',
      'Main Task',
      'Main Task',
      'Sidecar Task',
      'Poststart Task',
      'Poststop Task',
    ]);

    assert.ok(Chart.tasks[0].isPrestartEphemeral);
    assert.ok(Chart.tasks[1].isPrestartSidecar);
    assert.ok(Chart.tasks[2].isMain);
    assert.ok(Chart.tasks[4].isPoststartSidecar);
    assert.ok(Chart.tasks[5].isPoststartEphemeral);
    assert.ok(Chart.tasks[6].isPoststop);

    Chart.tasks.forEach((task) => {
      assert.notOk(task.isActive);
      assert.notOk(task.isFinished);
    });

    await componentA11yAudit(this.element, assert);
  });

  test('it doesn’t render when there’s only one phase', async function (assert) {
    this.set('tasks', [
      {
        lifecycleName: 'main',
      },
    ]);

    await render(hbs`<LifecycleChart @tasks={{tasks}} />`);
    assert.notOk(Chart.isPresent);
  });

  test('it renders all phases when there are any non-main tasks', async function (assert) {
    this.set('tasks', [tasks[0], tasks[6]]);

    await render(hbs`<LifecycleChart @tasks={{tasks}} />`);
    assert.equal(Chart.phases.length, 4);
  });

  test('it reflects phase and task states when states are passed in', async function (assert) {
    assert.expect(24);

    this.set(
      'taskStates',
      tasks.map((task) => {
        return { task };
      })
    );

    await render(hbs`<LifecycleChart @taskStates={{taskStates}} />`);
    assert.ok(Chart.isPresent);

    Chart.phases.forEach((phase) => assert.notOk(phase.isActive));

    Chart.tasks.forEach((task) => {
      assert.notOk(task.isActive);
      assert.notOk(task.isFinished);
    });

    // Change poststart-ephemeral to be running
    this.set('taskStates.4.state', 'running');
    await settled();

    await componentA11yAudit(this.element, assert);

    assert.ok(Chart.tasks[5].isActive);

    assert.ok(Chart.phases[1].isActive);
    assert.notOk(
      Chart.phases[2].isActive,
      'the poststart phase is nested within main and should never have the active class'
    );

    this.set('taskStates.4.finishedAt', new Date());
    this.set('taskStates.4.state', 'dead');
    await settled();

    assert.ok(Chart.tasks[5].isFinished);
  });

  [
    {
      testName: 'expected active phases',
      runningTaskNames: ['prestart ephemeral', 'main one', 'poststop'],
      activePhaseNames: ['Prestart', 'Main', 'Poststop'],
    },
    {
      testName: 'sidecar task states don’t affect phase active states',
      runningTaskNames: ['prestart sidecar', 'poststart sidecar'],
      activePhaseNames: [],
    },
    {
      testName:
        'poststart ephemeral task states affect main phase active state',
      runningTaskNames: ['poststart ephemeral'],
      activePhaseNames: ['Main'],
    },
  ].forEach(async ({ testName, runningTaskNames, activePhaseNames }) => {
    test(testName, async function (assert) {
      assert.expect(4);

      this.set(
        'taskStates',
        tasks.map((task) => ({ task }))
      );

      await render(hbs`<LifecycleChart @taskStates={{taskStates}} />`);

      runningTaskNames.forEach((taskName) => {
        const taskState = this.taskStates.find((taskState) =>
          taskState.task.name.includes(taskName)
        );
        set(taskState, 'state', 'running');
      });

      await settled();

      Chart.phases.forEach((Phase) => {
        if (activePhaseNames.includes(Phase.name)) {
          assert.ok(Phase.isActive, `expected ${Phase.name} not to be active`);
        } else {
          assert.notOk(
            Phase.isActive,
            `expected ${Phase.name} phase not to be active`
          );
        }
      });
    });
  });
});
