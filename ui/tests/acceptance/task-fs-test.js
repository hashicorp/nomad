import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import moment from 'moment';

import setupMirage from 'ember-cli-mirage/test-support/setup-mirage';
import Response from 'ember-cli-mirage/response';

import { formatBytes } from 'nomad-ui/helpers/format-bytes';
import { filesForPath } from 'nomad-ui/mirage/config';

import browseFilesystem from './behaviors/fs';

import FS from 'nomad-ui/tests/pages/allocations/task/fs';

let allocation;
let task;
let files, taskDirectory, directory, nestedDirectory;

const fileSort = (prop, files) => {
  let dir = [];
  let file = [];
  files.forEach(f => {
    if (f.isDir) {
      dir.push(f);
    } else {
      file.push(f);
    }
  });

  return dir.sortBy(prop).concat(file.sortBy(prop));
};

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

    this.task = task;
    this.allocation = allocation;

    // Reset files
    files = [];

    taskDirectory = server.create('allocFile', { isDir: true, name: task.name });
    files.push(taskDirectory);

    // Nested files
    directory = server.create('allocFile', { isDir: true, name: 'directory', parent: taskDirectory });
    files.push(directory);

    nestedDirectory = server.create('allocFile', { isDir: true, name: 'another', parent: directory });
    files.push(nestedDirectory);

    files.push(
      server.create('allocFile', 'file', {
        name: 'something.txt',
        fileType: 'txt',
        parent: nestedDirectory,
      })
    );

    files.push(server.create('allocFile', { isDir: true, name: 'empty-directory', parent: taskDirectory }));
    files.push(server.create('allocFile', 'file', { fileType: 'txt', parent: taskDirectory }));
    files.push(server.create('allocFile', 'file', { fileType: 'txt', parent: taskDirectory }));

    this.files = files;
    this.directory = directory;
  });

  test('visiting /allocations/:allocation_id/:task_name/fs', async function(assert) {
    await FS.visit({ id: allocation.id, name: task.name });
    assert.equal(currentURL(), `/allocations/${allocation.id}/${task.name}/fs`, 'No redirect');
  });

  test('when the task is not running, an empty state is shown', async function(assert) {
    // The API 500s on stat when not running
    this.server.get('/client/fs/stat/:allocation_id', () => {
      return new Response(500, {}, 'no such file or directory');
    });

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

  browseFilesystem({
    visitSegments: ({allocation,task}) => ({ id: allocation.id, name: task.name }),
    getExpectedPathBase: ({allocation,task}) => `/allocations/${allocation.id}/${task.name}/fs/`,
    getTitleComponent: ({task}) => `Task ${task.name} filesystem`,
    getBreadcrumbComponent: ({task}) => task.name,
    getFilesystemRoot: ({ task }) => task.name,
    pageObjectVisitPathFunctionName: 'visitPath',
  });

  test('navigating allocation filesystem', async function(assert) {
    await FS.visitPath({ id: allocation.id, name: task.name, path: '/' });

    const sortedFiles = fileSort('name', filesForPath(this.server.schema.allocFiles, task.name).models);

    assert.ok(FS.fileViewer.isHidden);

    assert.equal(FS.directoryEntries.length, 4);

    assert.equal(FS.breadcrumbsText, task.name);

    assert.equal(FS.breadcrumbs.length, 1);
    assert.ok(FS.breadcrumbs[0].isActive);
    assert.equal(FS.breadcrumbs[0].text, 'task-name');

    FS.directoryEntries[0].as(directory => {
      const fileRecord = sortedFiles[0];
      assert.equal(directory.name, fileRecord.name, 'directories should come first');
      assert.ok(directory.isDirectory);
      assert.equal(directory.size, '', 'directory sizes are hidden');
      assert.equal(directory.lastModified, moment(fileRecord.modTime).fromNow());
      assert.notOk(directory.path.includes('//'), 'paths shouldn’t have redundant separators');
    });

    FS.directoryEntries[2].as(file => {
      const fileRecord = sortedFiles[2];
      assert.equal(file.name, fileRecord.name);
      assert.ok(file.isFile);
      assert.equal(file.size, formatBytes([fileRecord.size]));
      assert.equal(file.lastModified, moment(fileRecord.modTime).fromNow());
    });

    await FS.directoryEntries[0].visit();

    assert.equal(FS.directoryEntries.length, 1);

    assert.equal(FS.breadcrumbs.length, 2);
    assert.equal(FS.breadcrumbsText, `${task.name} ${directory.name}`);

    assert.notOk(FS.breadcrumbs[0].isActive);

    assert.equal(FS.breadcrumbs[1].text, directory.name);
    assert.ok(FS.breadcrumbs[1].isActive);

    await FS.directoryEntries[0].visit();

    assert.equal(FS.directoryEntries.length, 1);
    assert.notOk(
      FS.directoryEntries[0].path.includes('//'),
      'paths shouldn’t have redundant separators'
    );

    assert.equal(FS.breadcrumbs.length, 3);
    assert.equal(FS.breadcrumbsText, `${task.name} ${directory.name} ${nestedDirectory.name}`);
    assert.equal(FS.breadcrumbs[2].text, nestedDirectory.name);

    assert.notOk(
      FS.breadcrumbs[0].path.includes('//'),
      'paths shouldn’t have redundant separators'
    );
    assert.notOk(
      FS.breadcrumbs[1].path.includes('//'),
      'paths shouldn’t have redundant separators'
    );

    await FS.breadcrumbs[1].visit();
    assert.equal(FS.breadcrumbsText, `${task.name} ${directory.name}`);
    assert.equal(FS.breadcrumbs.length, 2);
  });
});
