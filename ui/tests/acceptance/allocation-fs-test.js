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
let allocationShortId;
let files;

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

module('Acceptance | allocation fs', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function() {
    server.create('agent');
    server.create('node', 'forceIPv4');
    const job = server.create('job', { createAllocations: false });

    allocation = server.create('allocation', { jobId: job.id, clientStatus: 'running' });
    allocationShortId = allocation.id.split('-')[0];

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
  });

  test('visiting /allocations/:allocation_id/fs', async function(assert) {
    await FS.visitAllocation({ id: allocation.id  });
    assert.equal(currentURL(), `/allocations/${allocation.id}/fs`, 'No redirect');
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
    pageObjectVisitPathFunctionName: 'visitAllocationPath',
  });

  test('navigating allocation filesystem', async function(assert) {
    await FS.visitAllocationPath({ id: allocation.id, path: '/' });

    const sortedFiles = fileSort('name', filesForPath(this.server.schema.allocFiles, '').models);

    assert.ok(FS.fileViewer.isHidden);

    assert.equal(FS.directoryEntries.length, 4);

    assert.equal(FS.breadcrumbsText, allocationShortId);

    assert.equal(FS.breadcrumbs.length, 1);
    assert.ok(FS.breadcrumbs[0].isActive);
    assert.equal(FS.breadcrumbs[0].text, allocationShortId);

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
    assert.equal(FS.breadcrumbsText, `${allocationShortId} ${files[0].name}`);

    assert.notOk(FS.breadcrumbs[0].isActive);

    assert.equal(FS.breadcrumbs[1].text, files[0].name);
    assert.ok(FS.breadcrumbs[1].isActive);

    await FS.directoryEntries[0].visit();

    assert.equal(FS.directoryEntries.length, 1);
    assert.notOk(
      FS.directoryEntries[0].path.includes('//'),
      'paths shouldn’t have redundant separators'
    );

    assert.equal(FS.breadcrumbs.length, 3);
    assert.equal(FS.breadcrumbsText, `${allocationShortId} ${files[0].name} ${files[1].name}`);
    assert.equal(FS.breadcrumbs[2].text, files[1].name);

    assert.notOk(
      FS.breadcrumbs[0].path.includes('//'),
      'paths shouldn’t have redundant separators'
    );
    assert.notOk(
      FS.breadcrumbs[1].path.includes('//'),
      'paths shouldn’t have redundant separators'
    );

    await FS.breadcrumbs[1].visit();
    assert.equal(FS.breadcrumbsText, `${allocationShortId} ${files[0].name}`);
    assert.equal(FS.breadcrumbs.length, 2);
  });
});
