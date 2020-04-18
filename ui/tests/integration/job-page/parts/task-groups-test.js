import { assign } from '@ember/polyfills';
import hbs from 'htmlbars-inline-precompile';
import { click, findAll, find } from '@ember/test-helpers';
import { module, test } from 'qunit';
import sinon from 'sinon';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { setupRenderingTest } from 'ember-qunit';

module('Integration | Component | job-page/parts/task-groups', function(hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function() {
    window.localStorage.clear();
    this.store = this.owner.lookup('service:store');
    this.server = startMirage();
    this.server.create('namespace');
  });

  hooks.afterEach(function() {
    this.server.shutdown();
  });

  const props = (job, options = {}) =>
    assign(
      {
        job,
        sortProperty: 'name',
        sortDescending: true,
        gotoTaskGroup: () => {},
      },
      options
    );

  test('the job detail page should list all task groups', async function(assert) {
    this.server.create('job', {
      createAllocations: false,
    });

    await this.store.findAll('job').then(jobs => {
      jobs.forEach(job => job.reload());
    });

    const job = this.store.peekAll('job').get('firstObject');
    this.setProperties(props(job));

    await this.render(hbs`
      {{job-page/parts/task-groups
        job=job
        sortProperty=sortProperty
        sortDescending=sortDescending
        gotoTaskGroup=gotoTaskGroup}}
    `);

    assert.equal(
      findAll('[data-test-task-group]').length,
      job.get('taskGroups.length'),
      'One row per task group'
    );
  });

  test('each row in the task group table should show basic information about the task group', async function(assert) {
    this.server.create('job', {
      createAllocations: false,
    });

    const job = await this.store.findAll('job').then(async jobs => {
      return await jobs.get('firstObject').reload();
    });

    const taskGroups = await job.get('taskGroups');
    const taskGroup = taskGroups
      .sortBy('name')
      .reverse()
      .get('firstObject');

    this.setProperties(props(job));

    await this.render(hbs`
      {{job-page/parts/task-groups
        job=job
        sortProperty=sortProperty
        sortDescending=sortDescending
        gotoTaskGroup=gotoTaskGroup}}
    `);

    const taskGroupRow = find('[data-test-task-group]');

    assert.equal(
      taskGroupRow.querySelector('[data-test-task-group-name]').textContent.trim(),
      taskGroup.get('name'),
      'Name'
    );
    assert.equal(
      taskGroupRow.querySelector('[data-test-task-group-count]').textContent.trim(),
      taskGroup.get('count'),
      'Count'
    );
    assert.equal(
      taskGroupRow.querySelector('[data-test-task-group-volume]').textContent.trim(),
      taskGroup.get('volumes.length') ? 'Yes' : '',
      'Volumes'
    );
    assert.equal(
      taskGroupRow.querySelector('[data-test-task-group-cpu]').textContent.trim(),
      `${taskGroup.get('reservedCPU')} MHz`,
      'Reserved CPU'
    );
    assert.equal(
      taskGroupRow.querySelector('[data-test-task-group-mem]').textContent.trim(),
      `${taskGroup.get('reservedMemory')} MiB`,
      'Reserved Memory'
    );
    assert.equal(
      taskGroupRow.querySelector('[data-test-task-group-disk]').textContent.trim(),
      `${taskGroup.get('reservedEphemeralDisk')} MiB`,
      'Reserved Disk'
    );
  });

  test('gotoTaskGroup is called when task group rows are clicked', async function(assert) {
    this.server.create('job', {
      createAllocations: false,
    });

    const job = await this.store.findAll('job').then(async jobs => {
      return await jobs.get('firstObject').reload();
    });

    const taskGroupSpy = sinon.spy();

    const taskGroups = await job.get('taskGroups');
    const taskGroup = taskGroups
      .sortBy('name')
      .reverse()
      .get('firstObject');

    this.setProperties(
      props(job, {
        gotoTaskGroup: taskGroupSpy,
      })
    );

    await this.render(hbs`
      {{job-page/parts/task-groups
        job=job
        sortProperty=sortProperty
        sortDescending=sortDescending
        gotoTaskGroup=gotoTaskGroup}}
    `);

    await click('[data-test-task-group]');

    assert.ok(
      taskGroupSpy.withArgs(taskGroup).calledOnce,
      'Clicking the task group row calls the gotoTaskGroup action'
    );
  });
});
