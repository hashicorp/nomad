/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember-a11y-testing/a11y-audit-called */ // Covered in behaviours/fs
import { module } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';

import setupMirage from 'ember-cli-mirage/test-support/setup-mirage';

import browseFilesystem from './behaviors/fs';

let allocation;
let task;
let files, taskDirectory, directory, nestedDirectory;

module('Acceptance | task fs', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    server.create('agent');
    server.create('node-pool');
    server.create('node', 'forceIPv4');
    const job = server.create('job', { createAllocations: false });

    allocation = server.create('allocation', {
      jobId: job.id,
      clientStatus: 'running',
    });
    task = server.schema.taskStates.where({ allocationId: allocation.id })
      .models[0];
    task.name = 'task-name';
    task.save();

    this.task = task;
    this.allocation = allocation;

    // Reset files
    files = [];

    taskDirectory = server.create('allocFile', {
      isDir: true,
      name: task.name,
    });
    files.push(taskDirectory);

    // Nested files
    directory = server.create('allocFile', {
      isDir: true,
      name: 'directory',
      parent: taskDirectory,
    });
    files.push(directory);

    nestedDirectory = server.create('allocFile', {
      isDir: true,
      name: 'another',
      parent: directory,
    });
    files.push(nestedDirectory);

    files.push(
      server.create('allocFile', 'file', {
        name: 'something.txt',
        fileType: 'txt',
        parent: nestedDirectory,
      })
    );

    files.push(
      server.create('allocFile', {
        isDir: true,
        name: 'empty-directory',
        parent: taskDirectory,
      })
    );
    files.push(
      server.create('allocFile', 'file', {
        fileType: 'txt',
        parent: taskDirectory,
      })
    );
    files.push(
      server.create('allocFile', 'file', {
        fileType: 'txt',
        parent: taskDirectory,
      })
    );

    this.files = files;
    this.directory = directory;
    this.nestedDirectory = nestedDirectory;
  });

  browseFilesystem({
    visitSegments: ({ allocation, task }) => ({
      id: allocation.id,
      name: task.name,
    }),
    getExpectedPathBase: ({ allocation, task }) =>
      `/allocations/${allocation.id}/${task.name}/fs/`,
    getTitleComponent: ({ task }) => `Task ${task.name} filesystem`,
    getBreadcrumbComponent: ({ task }) => task.name,
    getFilesystemRoot: ({ task }) => task.name,
    pageObjectVisitFunctionName: 'visitTask',
    pageObjectVisitPathFunctionName: 'visitTaskPath',
  });
});
