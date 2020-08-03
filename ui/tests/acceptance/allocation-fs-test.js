/* eslint-disable ember-a11y-testing/a11y-audit-called */ // Covered in behaviours/fs
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';

import setupMirage from 'ember-cli-mirage/test-support/setup-mirage';
import Response from 'ember-cli-mirage/response';

import browseFilesystem from './behaviors/fs';

import FS from 'nomad-ui/tests/pages/allocations/fs';

let allocation;
let files;

module('Acceptance | allocation fs', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function() {
    server.create('agent');
    server.create('node', 'forceIPv4');
    const job = server.create('job', { createAllocations: false });

    allocation = server.create('allocation', { jobId: job.id, clientStatus: 'running' });

    this.allocation = allocation;

    // Reset files
    files = [];

    // Nested files
    files.push(server.create('allocFile', { isDir: true, name: 'directory' }));
    files.push(server.create('allocFile', { isDir: true, name: 'another', parent: files[0] }));
    files.push(
      server.create('allocFile', 'file', {
        name: 'something.txt',
        fileType: 'txt',
        parent: files[1],
      })
    );

    files.push(server.create('allocFile', { isDir: true, name: 'empty-directory' }));
    files.push(server.create('allocFile', 'file', { fileType: 'txt' }));
    files.push(server.create('allocFile', 'file', { fileType: 'txt' }));

    this.files = files;
    this.directory = files[0];
    this.nestedDirectory = files[1];
  });

  test('when the allocation is not running, an empty state is shown', async function(assert) {
    // The API 500s on stat when not running
    this.server.get('/client/fs/stat/:allocation_id', () => {
      return new Response(500, {}, 'no such file or directory');
    });

    allocation.update({
      clientStatus: 'complete',
    });

    await FS.visitAllocation({ id: allocation.id });
    assert.ok(FS.hasEmptyState, 'Non-running allocation has no files');
    assert.ok(
      FS.emptyState.headline.includes('Allocation is not Running'),
      'Empty state explains the condition'
    );
  });

  browseFilesystem({
    visitSegments: ({ allocation }) => ({ id: allocation.id }),
    getExpectedPathBase: ({ allocation }) => `/allocations/${allocation.id}/fs/`,
    getTitleComponent: ({ allocation }) => `Allocation ${allocation.id.split('-')[0]} filesystem`,
    getBreadcrumbComponent: ({ allocation }) => allocation.id.split('-')[0],
    getFilesystemRoot: () => '',
    pageObjectVisitFunctionName: 'visitAllocation',
    pageObjectVisitPathFunctionName: 'visitAllocationPath',
  });
});
