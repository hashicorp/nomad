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
let files;

module('Acceptance | allocation fs', function (hooks) {
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

    this.allocation = allocation;

    // Reset files
    files = [];

    // Nested files
    files.push(server.create('allocFile', { isDir: true, name: 'directory' }));
    files.push(
      server.create('allocFile', {
        isDir: true,
        name: 'another',
        parent: files[0],
      })
    );
    files.push(
      server.create('allocFile', 'file', {
        name: 'something.txt',
        fileType: 'txt',
        parent: files[1],
      })
    );

    files.push(
      server.create('allocFile', { isDir: true, name: 'empty-directory' })
    );
    files.push(server.create('allocFile', 'file', { fileType: 'txt' }));
    files.push(server.create('allocFile', 'file', { fileType: 'txt' }));

    this.files = files;
    this.directory = files[0];
    this.nestedDirectory = files[1];
  });

  browseFilesystem({
    visitSegments: ({ allocation }) => ({ id: allocation.id }),
    getExpectedPathBase: ({ allocation }) =>
      `/allocations/${allocation.id}/fs/`,
    getTitleComponent: ({ allocation }) =>
      `Allocation ${allocation.id.split('-')[0]} filesystem`,
    getBreadcrumbComponent: ({ allocation }) => allocation.id.split('-')[0],
    getFilesystemRoot: () => '',
    pageObjectVisitFunctionName: 'visitAllocation',
    pageObjectVisitPathFunctionName: 'visitAllocationPath',
  });
});
