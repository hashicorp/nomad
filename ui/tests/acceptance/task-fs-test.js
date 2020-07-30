/* eslint-disable ember-a11y-testing/a11y-audit-called */ // Covered in behaviours/fs
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';

import setupMirage from 'ember-cli-mirage/test-support/setup-mirage';
import Response from 'ember-cli-mirage/response';

import browseFilesystem from './behaviors/fs';

import FS from 'nomad-ui/tests/pages/allocations/fs';

let allocation;
let task;
let files, taskDirectory, directory, nestedDirectory;

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
    this.nestedDirectory = nestedDirectory;
  });

  test('when the task is not running, an empty state is shown', async function(assert) {
    // The API 500s on stat when not running
    this.server.get('/client/fs/stat/:allocation_id', () => {
      return new Response(500, {}, 'no such file or directory');
    });

    task.update({
      finishedAt: new Date(),
    });

    await FS.visitTask({ id: allocation.id, name: task.name });
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
    pageObjectVisitFunctionName: 'visitTask',
    pageObjectVisitPathFunctionName: 'visitTaskPath',
  });
});
