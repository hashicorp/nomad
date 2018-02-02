import { getOwner } from '@ember/application';
import { assign } from '@ember/polyfills';
import hbs from 'htmlbars-inline-precompile';
import wait from 'ember-test-helpers/wait';
import { click, findAll, find } from 'ember-native-dom-helpers';
import { test, moduleForComponent } from 'ember-qunit';
import sinon from 'sinon';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';

moduleForComponent(
  'job-page/parts/task-groups',
  'Integration | Component | job-page/parts/task-groups',
  {
    integration: true,
    beforeEach() {
      fragmentSerializerInitializer(getOwner(this));
      window.localStorage.clear();
      this.store = getOwner(this).lookup('service:store');
      this.server = startMirage();
      this.server.create('namespace');
    },
    afterEach() {
      this.server.shutdown();
    },
  }
);

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

test('the job detail page should list all task groups', function(assert) {
  this.server.create('job', {
    createAllocations: false,
  });

  this.store.findAll('job').then(jobs => {
    jobs.forEach(job => job.reload());
  });

  return wait().then(() => {
    const job = this.store.peekAll('job').get('firstObject');
    this.setProperties(props(job));

    this.render(hbs`
      {{job-page/parts/task-groups
        job=job
        sortProperty=sortProperty
        sortDescending=sortDescending
        gotoTaskGroup=gotoTaskGroup}}
    `);

    return wait().then(() => {
      assert.equal(
        findAll('[data-test-task-group]').length,
        job.get('taskGroups.length'),
        'One row per task group'
      );
    });
  });
});

test('each row in the task group table should show basic information about the task group', function(assert) {
  this.server.create('job', {
    createAllocations: false,
  });

  this.store.findAll('job').then(jobs => {
    jobs.forEach(job => job.reload());
  });

  return wait().then(() => {
    const job = this.store.peekAll('job').get('firstObject');
    const taskGroup = job
      .get('taskGroups')
      .sortBy('name')
      .reverse()
      .get('firstObject');

    this.setProperties(props(job));

    this.render(hbs`
      {{job-page/parts/task-groups
        job=job
        sortProperty=sortProperty
        sortDescending=sortDescending
        gotoTaskGroup=gotoTaskGroup}}
    `);

    return wait().then(() => {
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
  });
});

test('gotoTaskGroup is called when task group rows are clicked', function(assert) {
  this.server.create('job', {
    createAllocations: false,
  });

  this.store.findAll('job').then(jobs => {
    jobs.forEach(job => job.reload());
  });

  return wait().then(() => {
    const taskGroupSpy = sinon.spy();
    const job = this.store.peekAll('job').get('firstObject');
    const taskGroup = job
      .get('taskGroups')
      .sortBy('name')
      .reverse()
      .get('firstObject');

    this.setProperties(
      props(job, {
        gotoTaskGroup: taskGroupSpy,
      })
    );

    this.render(hbs`
      {{job-page/parts/task-groups
        job=job
        sortProperty=sortProperty
        sortDescending=sortDescending
        gotoTaskGroup=gotoTaskGroup}}
    `);

    return wait().then(() => {
      click('[data-test-task-group]');
      assert.ok(
        taskGroupSpy.withArgs(taskGroup).calledOnce,
        'Clicking the task group row calls the gotoTaskGroup action'
      );
    });
  });
});
