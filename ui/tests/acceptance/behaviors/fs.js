/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { test } from 'qunit';
import { getPageTitle } from 'ember-page-title/test-support';
import { currentURL, visit } from '@ember/test-helpers';

import { filesForPath } from 'nomad-ui/mirage/config';
import { formatBytes } from 'nomad-ui/utils/units';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';

import { Response } from 'miragejs';
import moment from 'moment';

import FS from 'nomad-ui/tests/pages/allocations/fs';

const fileSort = (prop, files) => {
  let dir = [];
  let file = [];
  files.forEach((f) => {
    if (f.isDir) {
      dir.push(f);
    } else {
      file.push(f);
    }
  });

  return dir.sortBy(prop).concat(file.sortBy(prop));
};

export default function browseFilesystem({
  pageObjectVisitPathFunctionName,
  pageObjectVisitFunctionName,
  visitSegments,
  getExpectedPathBase,
  getTitleComponent,
  getBreadcrumbComponent,
  getFilesystemRoot,
}) {
  test('it passes an accessibility audit', async function (assert) {
    await FS[pageObjectVisitFunctionName](
      visitSegments({ allocation: this.allocation, task: this.task }),
    );
    await a11yAudit(assert);
  });

  test('visiting filesystem root', async function (assert) {
    await FS[pageObjectVisitFunctionName](
      visitSegments({ allocation: this.allocation, task: this.task }),
    );

    const pathBaseWithTrailingSlash = getExpectedPathBase({
      allocation: this.allocation,
      task: this.task,
    });
    const pathBaseWithoutTrailingSlash = pathBaseWithTrailingSlash.slice(0, -1);

    assert.deepEqual(currentURL(), pathBaseWithoutTrailingSlash, 'No redirect');
  });

  test('visiting filesystem paths', async function (assert) {
    const paths = [
      'some-file.log',
      'a/deep/path/to/a/file.log',
      '/',
      'Unicode™®',
    ];

    const testPath = async (filePath) => {
      let pathWithLeadingSlash = filePath;

      if (!pathWithLeadingSlash.startsWith('/')) {
        pathWithLeadingSlash = `/${filePath}`;
      }

      await FS[pageObjectVisitPathFunctionName]({
        ...visitSegments({ allocation: this.allocation, task: this.task }),
        path: filePath,
      });
      assert.deepEqual(
        currentURL(),
        `${getExpectedPathBase({
          allocation: this.allocation,
          task: this.task,
        })}${encodeURIComponent(filePath)}`,
        'No redirect',
      );
      assert.ok(
        getPageTitle().includes(
          `${pathWithLeadingSlash} - ${getTitleComponent({
            allocation: this.allocation,
            task: this.task,
          })}`,
        ),
      );
      assert.deepEqual(
        FS.breadcrumbsText,
        `${getBreadcrumbComponent({
          allocation: this.allocation,
          task: this.task,
        })} ${filePath.replace(/\//g, ' ')}`.trim(),
      );
    };

    await paths.reduce(async (prev, filePath) => {
      await prev;
      return testPath(filePath);
    }, Promise.resolve());
  });

  test('navigating allocation filesystem', async function (assert) {
    const objects = { allocation: this.allocation, task: this.task };
    await FS[pageObjectVisitPathFunctionName]({
      ...visitSegments(objects),
      path: '/',
    });

    const sortedFiles = fileSort(
      'name',
      filesForPath(this.server.schema.allocFiles, getFilesystemRoot(objects))
        .models,
    );

    assert.ok(FS.fileViewer.isHidden);

    assert.deepEqual(FS.directoryEntries.length, 4);

    assert.deepEqual(FS.breadcrumbsText, getBreadcrumbComponent(objects));

    assert.deepEqual(FS.breadcrumbs.length, 1);
    assert.ok(FS.breadcrumbs[0].isActive);
    assert.deepEqual(FS.breadcrumbs[0].text, getBreadcrumbComponent(objects));

    FS.directoryEntries[0].as((directory) => {
      const fileRecord = sortedFiles[0];
      assert.deepEqual(
        directory.name,
        fileRecord.name,
        'directories should come first',
      );
      assert.ok(directory.isDirectory);
      assert.deepEqual(directory.size, '', 'directory sizes are hidden');
      assert.deepEqual(
        directory.lastModified,
        moment(fileRecord.modTime).fromNow(),
      );
      assert.notOk(
        directory.path.includes('//'),
        'paths shouldn’t have redundant separators',
      );
    });

    FS.directoryEntries[2].as((file) => {
      const fileRecord = sortedFiles[2];
      assert.deepEqual(file.name, fileRecord.name);
      assert.ok(file.isFile);
      assert.deepEqual(file.size, formatBytes(fileRecord.size));
      assert.deepEqual(file.lastModified, moment(fileRecord.modTime).fromNow());
    });

    await FS.directoryEntries[0].visit();

    assert.deepEqual(FS.directoryEntries.length, 1);

    assert.deepEqual(FS.breadcrumbs.length, 2);
    assert.deepEqual(
      FS.breadcrumbsText,
      `${getBreadcrumbComponent(objects)} ${this.directory.name}`,
    );

    assert.notOk(FS.breadcrumbs[0].isActive);

    assert.deepEqual(FS.breadcrumbs[1].text, this.directory.name);
    assert.ok(FS.breadcrumbs[1].isActive);

    await FS.directoryEntries[0].visit();

    assert.deepEqual(FS.directoryEntries.length, 1);
    assert.notOk(
      FS.directoryEntries[0].path.includes('//'),
      'paths shouldn’t have redundant separators',
    );

    assert.deepEqual(FS.breadcrumbs.length, 3);
    assert.deepEqual(
      FS.breadcrumbsText,
      `${getBreadcrumbComponent(objects)} ${this.directory.name} ${
        this.nestedDirectory.name
      }`,
    );
    assert.deepEqual(FS.breadcrumbs[2].text, this.nestedDirectory.name);

    assert.notOk(
      FS.breadcrumbs[0].path.includes('//'),
      'paths shouldn’t have redundant separators',
    );
    assert.notOk(
      FS.breadcrumbs[1].path.includes('//'),
      'paths shouldn’t have redundant separators',
    );

    await FS.breadcrumbs[1].visit();
    assert.deepEqual(
      FS.breadcrumbsText,
      `${getBreadcrumbComponent(objects)} ${this.directory.name}`,
    );
    assert.deepEqual(FS.breadcrumbs.length, 2);
  });

  test('sorting allocation filesystem directory', async function (assert) {
    this.server.get('/client/fs/ls/:allocation_id', () => {
      return [
        {
          Name: 'aaa-big-old-file',
          IsDir: false,
          Size: 19190000,
          ModTime: moment().subtract(1, 'year').format(),
        },
        {
          Name: 'mmm-small-mid-file',
          IsDir: false,
          Size: 1919,
          ModTime: moment().subtract(6, 'month').format(),
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
          ModTime: moment().subtract(1, 'year').format(),
        },
        {
          Name: 'mmm-small-mid-directory',
          IsDir: true,
          Size: 1919,
          ModTime: moment().subtract(6, 'month').format(),
        },
        {
          Name: 'zzz-med-new-directory',
          IsDir: true,
          Size: 191900,
          ModTime: moment().format(),
        },
      ];
    });

    await FS[pageObjectVisitPathFunctionName]({
      ...visitSegments({ allocation: this.allocation, task: this.task }),
      path: '/',
    });

    assert.deepEqual(FS.directoryEntryNames(), [
      'aaa-big-old-directory',
      'mmm-small-mid-directory',
      'zzz-med-new-directory',
      'aaa-big-old-file',
      'mmm-small-mid-file',
      'zzz-med-new-file',
    ]);

    await FS.sortBy('Name');

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

    await FS.sortBy('Size');

    assert.deepEqual(
      FS.directoryEntryNames(),
      [
        'aaa-big-old-file',
        'zzz-med-new-file',
        'mmm-small-mid-file',
        'zzz-med-new-directory',
        'mmm-small-mid-directory',
        'aaa-big-old-directory',
      ],
      'expected files to be sorted by descending size and directories to be sorted by descending name',
    );

    await FS.sortBy('Size');

    assert.deepEqual(
      FS.directoryEntryNames(),
      [
        'aaa-big-old-directory',
        'mmm-small-mid-directory',
        'zzz-med-new-directory',
        'mmm-small-mid-file',
        'zzz-med-new-file',
        'aaa-big-old-file',
      ],
      'expected directories to be sorted by name and files to be sorted by ascending size',
    );
  });

  test('viewing a file', async function (assert) {
    const objects = { allocation: this.allocation, task: this.task };
    const node = this.server.db.nodes.find(this.allocation.nodeId);

    this.server.get(
      `http://${node.httpAddr}/v1/client/fs/readat/:allocation_id`,
      function () {
        return new Response(500);
      },
    );

    await FS[pageObjectVisitPathFunctionName]({
      ...visitSegments(objects),
      path: '/',
    });

    const sortedFiles = fileSort(
      'name',
      filesForPath(this.server.schema.allocFiles, getFilesystemRoot(objects))
        .models,
    );
    const fileRecord = sortedFiles.find((f) => !f.isDir);
    const fileIndex = sortedFiles.indexOf(fileRecord);

    await FS.directoryEntries[fileIndex].visit();

    assert.deepEqual(
      FS.breadcrumbsText,
      `${getBreadcrumbComponent(objects)} ${fileRecord.name}`,
    );

    assert.ok(FS.fileViewer.isPresent);

    const readAtRequests = this.server.pretender.handledRequests.filter((req) =>
      req.url.includes(`/v1/client/fs/readat/${this.allocation.id}`),
    );
    const firstAttempt = readAtRequests[0];
    const secondAttempt = readAtRequests[1];

    assert.deepEqual(readAtRequests.length, 2, 'Two readat attempts were made');

    assert.deepEqual(
      firstAttempt.url.split('?')[0],
      `//${node.httpAddr}/v1/client/fs/readat/${this.allocation.id}`,
      'Client is hit first',
    );
    assert.deepEqual(firstAttempt.status, 500, 'Client request fails');
    assert.deepEqual(
      secondAttempt.url.split('?')[0],
      `/v1/client/fs/readat/${this.allocation.id}`,
      'Server is hit second',
    );
  });

  test('viewing an empty directory', async function (assert) {
    await FS[pageObjectVisitPathFunctionName]({
      ...visitSegments({ allocation: this.allocation, task: this.task }),
      path: 'empty-directory',
    });

    assert.ok(FS.isEmptyDirectory);
  });

  test('viewing paths that produce stat API errors', async function (assert) {
    this.server.get('/client/fs/stat/:allocation_id', () => {
      return new Response(500, {}, 'no such file or directory');
    });

    await FS[pageObjectVisitPathFunctionName]({
      ...visitSegments({ allocation: this.allocation, task: this.task }),
      path: '/what-is-this',
    });
    assert.notEqual(
      FS.error.title,
      'Not Found',
      '500 is not interpreted as 404',
    );
    assert.deepEqual(
      FS.error.title,
      'Server Error',
      '500 is not interpreted as 500',
    );

    await visit('/');

    this.server.get('/client/fs/stat/:allocation_id', () => {
      return new Response(599);
    });

    await FS[pageObjectVisitPathFunctionName]({
      ...visitSegments({ allocation: this.allocation, task: this.task }),
      path: '/what-is-this',
    });
    assert.deepEqual(
      FS.error.title,
      'Error',
      'other statuses are passed through',
    );
  });

  test('viewing paths that produce ls API errors', async function (assert) {
    this.server.get('/client/fs/ls/:allocation_id', () => {
      return new Response(500, {}, 'no such file or directory');
    });

    await FS[pageObjectVisitPathFunctionName]({
      ...visitSegments({ allocation: this.allocation, task: this.task }),
      path: this.directory.name,
    });
    assert.notEqual(
      FS.error.title,
      'Not Found',
      '500 is not interpreted as 404',
    );
    assert.deepEqual(
      FS.error.title,
      'Server Error',
      '500 is not interpreted as 404',
    );

    await visit('/');

    this.server.get('/client/fs/ls/:allocation_id', () => {
      return new Response(599);
    });

    await FS[pageObjectVisitPathFunctionName]({
      ...visitSegments({ allocation: this.allocation, task: this.task }),
      path: this.directory.name,
    });
    assert.deepEqual(
      FS.error.title,
      'Error',
      'other statuses are passed through',
    );
  });
}
