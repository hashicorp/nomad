import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, settled } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { create } from 'ember-cli-page-object';
import LifecycleChart from 'nomad-ui/tests/pages/components/lifecycle-chart';

const Chart = create(LifecycleChart);

const tasks = [
  {
    lifecycleName: 'main',
    name: 'main two',
  },
  {
    lifecycleName: 'main',
    name: 'main one',
  },
  {
    lifecycleName: 'prestart',
    name: 'prestart',
  },
  {
    lifecycleName: 'sidecar',
    name: 'sidecar',
  },
  {
    lifecycleName: 'poststop',
    name: 'poststop',
  },
];

module('Integration | Component | lifecycle-chart', function(hooks) {
  setupRenderingTest(hooks);

  test('it renders stateless phases and lifecycle- and name-sorted tasks', async function(assert) {
    this.set('tasks', tasks);

    await render(hbs`{{lifecycle-chart tasks=tasks}}`);
    assert.ok(Chart.isPresent);

    assert.equal(Chart.phases[0].name, 'Prestart');
    assert.equal(Chart.phases[1].name, 'Main');
    assert.equal(Chart.phases[2].name, 'Poststop');

    Chart.phases.forEach(phase => assert.notOk(phase.isActive));

    assert.deepEqual(Chart.tasks.mapBy('name'), [
      'prestart',
      'sidecar',
      'main one',
      'main two',
      'poststop',
    ]);
    assert.deepEqual(Chart.tasks.mapBy('lifecycle'), [
      'Prestart Task',
      'Sidecar Task',
      'Main Task',
      'Main Task',
      'Poststop Task',
    ]);

    assert.ok(Chart.tasks[0].isPrestart);
    assert.ok(Chart.tasks[1].isSidecar);
    assert.ok(Chart.tasks[2].isMain);
    assert.ok(Chart.tasks[4].isPoststop);

    Chart.tasks.forEach(task => {
      assert.notOk(task.isActive);
      assert.notOk(task.isFinished);
    });
  });

  test('it doesn’t render when there’s only one phase', async function(assert) {
    this.set('tasks', [
      {
        lifecycleName: 'main',
      },
    ]);

    await render(hbs`{{lifecycle-chart tasks=tasks}}`);
    assert.notOk(Chart.isPresent);
  });

  test('it reflects phase and task states when states are passed in', async function(assert) {
    this.set(
      'taskStates',
      tasks.map(task => {
        return { task };
      })
    );

    await render(hbs`{{lifecycle-chart taskStates=taskStates}}`);
    assert.ok(Chart.isPresent);

    Chart.phases.forEach(phase => assert.notOk(phase.isActive));

    Chart.tasks.forEach(task => {
      assert.notOk(task.isActive);
      assert.notOk(task.isFinished);
    });

    this.set('taskStates.firstObject.state', 'running');
    await settled();

    assert.ok(Chart.phases[1].isActive);
    assert.ok(Chart.tasks[3].isActive);

    this.set('taskStates.firstObject.finishedAt', new Date());
    await settled();

    assert.ok(Chart.tasks[3].isFinished);
  });
});
