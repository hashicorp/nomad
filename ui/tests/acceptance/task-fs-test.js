import { currentURL, visit } from '@ember/test-helpers';
import { Promise } from 'rsvp';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';

import setupMirage from 'ember-cli-mirage/test-support/setup-mirage';
import Response from 'ember-cli-mirage/response';
import moment from 'moment';

import FS from 'nomad-ui/tests/pages/allocations/task/fs';

let allocation;
let task;

module('Acceptance | task fs', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function() {
    server.create('agent');
    server.create('node', 'forceIPv4');
    const job = server.create('job', { createAllocations: false });

    allocation = server.create('allocation', { jobId: job.id, clientStatus: 'running' });
    task = server.schema.taskStates.where({ allocationId: allocation.id }).models[0];
    task.name = 'task-name';
    task.save();
  });

  test('visiting /allocations/:allocation_id/:task_name/fs', async function(assert) {
    await FS.visit({ id: allocation.id, name: task.name });
    assert.equal(currentURL(), `/allocations/${allocation.id}/${task.name}/fs`, 'No redirect');
  });

  test('when the task is not running, an empty state is shown', async function(assert) {
    task.update({
      finishedAt: new Date(),
    });

    await FS.visit({ id: allocation.id, name: task.name });
    assert.ok(FS.hasEmptyState, 'Non-running task has no files');
    assert.ok(
      FS.emptyState.headline.includes('Task is not Running'),
      'Empty state explains the condition'
    );
  });

  test('visiting /allocations/:allocation_id/:task_name/fs/:path', async function(assert) {
    const paths = ['some-file.log', 'a/deep/path/to/a/file.log', '/', 'Unicode™®'];

    const testPath = async filePath => {
      await FS.visitPath({ id: allocation.id, name: task.name, path: filePath });
      assert.equal(
        currentURL(),
        `/allocations/${allocation.id}/${task.name}/fs/${encodeURIComponent(filePath)}`,
        'No redirect'
      );
      assert.equal(FS.breadcrumbsText, `${task.name} ${filePath.replace(/\//g, ' ')}`.trim());
    };

    await paths.reduce(async (prev, filePath) => {
      await prev;
      return testPath(filePath);
    }, Promise.resolve());
  });

  test('navigating allocation filesystem', async function(assert) {
    await FS.visitPath({ id: allocation.id, name: task.name, path: '/' });

    assert.ok(FS.fileViewer.isHidden);

    assert.equal(FS.directoryEntries.length, 4);

    assert.equal(FS.breadcrumbsText, task.name);

    assert.equal(FS.breadcrumbs.length, 1);
    assert.ok(FS.breadcrumbs[0].isActive);
    assert.equal(FS.breadcrumbs[0].text, 'task-name');

    FS.directoryEntries[0].as(directory => {
      assert.equal(directory.name, 'directory', 'directories should come first');
      assert.ok(directory.isDirectory);
      assert.equal(directory.size, '', 'directory sizes are hidden');
      assert.equal(directory.lastModified, 'a year ago');
      assert.notOk(directory.path.includes('//'), 'paths shouldn’t have redundant separators');
    });

    FS.directoryEntries[2].as(file => {
      assert.equal(file.name, '🤩.txt');
      assert.ok(file.isFile);
      assert.equal(file.size, '1 KiB');
      assert.equal(file.lastModified, '2 days ago');
    });

    await FS.directoryEntries[0].visit();

    assert.equal(FS.directoryEntries.length, 1);

    assert.equal(FS.breadcrumbs.length, 2);
    assert.equal(FS.breadcrumbsText, 'task-name directory');

    assert.notOk(FS.breadcrumbs[0].isActive);

    assert.equal(FS.breadcrumbs[1].text, 'directory');
    assert.ok(FS.breadcrumbs[1].isActive);

    await FS.directoryEntries[0].visit();

    assert.equal(FS.directoryEntries.length, 1);
    assert.notOk(
      FS.directoryEntries[0].path.includes('//'),
      'paths shouldn’t have redundant separators'
    );

    assert.equal(FS.breadcrumbs.length, 3);
    assert.equal(FS.breadcrumbsText, 'task-name directory another');
    assert.equal(FS.breadcrumbs[2].text, 'another');

    assert.notOk(
      FS.breadcrumbs[0].path.includes('//'),
      'paths shouldn’t have redundant separators'
    );
    assert.notOk(
      FS.breadcrumbs[1].path.includes('//'),
      'paths shouldn’t have redundant separators'
    );

    await FS.breadcrumbs[1].visit();
    assert.equal(FS.breadcrumbsText, 'task-name directory');
    assert.equal(FS.breadcrumbs.length, 2);
  });

  test('sorting allocation filesystem directory', async function(assert) {
    this.server.get('/client/fs/ls/:allocation_id', () => {
      return [
        {
          Name: 'aaa-big-old-file',
          IsDir: false,
          Size: 19190000,
          ModTime: moment()
            .subtract(1, 'year')
            .format(),
        },
        {
          Name: 'mmm-small-mid-file',
          IsDir: false,
          Size: 1919,
          ModTime: moment()
            .subtract(6, 'month')
            .format(),
        },
        {
          Name: 'zzz-med-new-file',
          IsDir: false,
          Size: 191900,
          ModTime: moment().format(),
        },
        {
          Name: 'aaa-big-old-directory',
          IsDir: true,
          Size: 19190000,
          ModTime: moment()
            .subtract(1, 'year')
            .format(),
        },
        {
          Name: 'mmm-small-mid-directory',
          IsDir: true,
          Size: 1919,
          ModTime: moment()
            .subtract(6, 'month')
            .format(),
        },
        {
          Name: 'zzz-med-new-directory',
          IsDir: true,
          Size: 191900,
          ModTime: moment().format(),
        },
      ];
    });

    await FS.visitPath({ id: allocation.id, name: task.name, path: '/' });

    assert.deepEqual(FS.directoryEntryNames(), [
      'aaa-big-old-directory',
      'mmm-small-mid-directory',
      'zzz-med-new-directory',
      'aaa-big-old-file',
      'mmm-small-mid-file',
      'zzz-med-new-file',
    ]);

    await FS.sortBy('name');

    assert.deepEqual(FS.directoryEntryNames(), [
      'zzz-med-new-file',
      'mmm-small-mid-file',
      'aaa-big-old-file',
      'zzz-med-new-directory',
      'mmm-small-mid-directory',
      'aaa-big-old-directory',
    ]);

    await FS.sortBy('ModTime');

    assert.deepEqual(FS.directoryEntryNames(), [
      'zzz-med-new-file',
      'mmm-small-mid-file',
      'aaa-big-old-file',
      'zzz-med-new-directory',
      'mmm-small-mid-directory',
      'aaa-big-old-directory',
    ]);

    await FS.sortBy('ModTime');

    assert.deepEqual(FS.directoryEntryNames(), [
      'aaa-big-old-directory',
      'mmm-small-mid-directory',
      'zzz-med-new-directory',
      'aaa-big-old-file',
      'mmm-small-mid-file',
      'zzz-med-new-file',
    ]);
  });

  test('viewing a file', async function(assert) {
    await FS.visitPath({ id: allocation.id, name: task.name, path: '/' });
    await FS.directoryEntries[2].visit();

    assert.equal(FS.breadcrumbsText, 'task-name 🤩.txt');

    assert.ok(FS.fileViewer.isPresent);
  });

  test('viewing an empty directory', async function(assert) {
    await FS.visitPath({ id: allocation.id, name: task.name, path: '/empty-directory' });

    assert.equal(FS.directoryEntries.length, 1);
    assert.ok(FS.directoryEntries[0].isEmpty);
  });

  test('viewing paths that produce stat API errors', async function(assert) {
    this.server.get('/client/fs/stat/:allocation_id', () => {
      return new Response(500, {}, 'no such file or directory');
    });

    await FS.visitPath({ id: allocation.id, name: task.name, path: '/what-is-this' });
    assert.equal(FS.error.title, 'Not Found', '500 is interpreted as 404');

    await visit('/');

    this.server.get('/client/fs/stat/:allocation_id', () => {
      return new Response(999);
    });

    await FS.visitPath({ id: allocation.id, name: task.name, path: '/what-is-this' });
    assert.equal(FS.error.title, 'Error', 'other statuses are passed through');
  });

  test('viewing paths that produce ls API errors', async function(assert) {
    this.server.get('/client/fs/ls/:allocation_id', () => {
      return new Response(500, {}, 'no such file or directory');
    });

    await FS.visitPath({ id: allocation.id, name: task.name, path: '/what-is-this' });
    assert.equal(FS.error.title, 'Not Found', '500 is interpreted as 404');

    await visit('/');

    this.server.get('/client/fs/ls/:allocation_id', () => {
      return new Response(999);
    });

    await FS.visitPath({ id: allocation.id, name: task.name, path: '/what-is-this' });
    assert.equal(FS.error.title, 'Error', 'other statuses are passed through');
  });
});
