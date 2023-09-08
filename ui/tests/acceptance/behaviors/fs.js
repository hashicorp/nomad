/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/require-expect */
import { test } from 'qunit';
import { currentURL, visit } from '@ember/test-helpers';

import { filesForPath } from 'nomad-ui/mirage/config';
import { formatBytes } from 'nomad-ui/utils/units';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';

import Response from 'ember-cli-mirage/response';
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
      visitSegments({ allocation: this.allocation, task: this.task })
    );
    await a11yAudit(assert);
  });

  test('visiting filesystem root', async function (assert) {
    await FS[pageObjectVisitFunctionName](
      visitSegments({ allocation: this.allocation, task: this.task })
    );

    const pathBaseWithTrailingSlash = getExpectedPathBase({
      allocation: this.allocation,
      task: this.task,
    });
    const pathBaseWithoutTrailingSlash = pathBaseWithTrailingSlash.slice(0, -1);

    assert.equal(currentURL(), pathBaseWithoutTrailingSlash, 'No redirect');
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
      assert.equal(
        currentURL(),
        `${getExpectedPathBase({
          allocation: this.allocation,
          task: this.task,
        })}${encodeURIComponent(filePath)}`,
        'No redirect'
      );
      assert.ok(
        document.title.includes(
          `${pathWithLeadingSlash} - ${getTitleComponent({
            allocation: this.allocation,
            task: this.task,
          })}`
        )
      );
      assert.equal(
        FS.breadcrumbsText,
        `${getBreadcrumbComponent({
          allocation: this.allocation,
          task: this.task,
        })} ${filePath.replace(/\//g, ' ')}`.trim()
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
        .models
    );

    assert.ok(FS.fileViewer.isHidden);

    assert.equal(FS.directoryEntries.length, 4);

    assert.equal(FS.breadcrumbsText, getBreadcrumbComponent(objects));

    assert.equal(FS.breadcrumbs.length, 1);
    assert.ok(FS.breadcrumbs[0].isActive);
    assert.equal(FS.breadcrumbs[0].text, getBreadcrumbComponent(objects));

    FS.directoryEntries[0].as((directory) => {
      const fileRecord = sortedFiles[0];
      assert.equal(
        directory.name,
        fileRecord.name,
        'directories should come first'
      );
      assert.ok(directory.isDirectory);
      assert.equal(directory.size, '', 'directory sizes are hidden');
      assert.equal(
        directory.lastModified,
        moment(fileRecord.modTime).fromNow()
      );
      assert.notOk(
        directory.path.includes('//'),
        'paths shouldn’t have redundant separators'
      );
    });

    FS.directoryEntries[2].as((file) => {
      const fileRecord = sortedFiles[2];
      assert.equal(file.name, fileRecord.name);
      assert.ok(file.isFile);
      assert.equal(file.size, formatBytes(fileRecord.size));
      assert.equal(file.lastModified, moment(fileRecord.modTime).fromNow());
    });

    await FS.directoryEntries[0].visit();

    assert.equal(FS.directoryEntries.length, 1);

    assert.equal(FS.breadcrumbs.length, 2);
    assert.equal(
      FS.breadcrumbsText,
      `${getBreadcrumbComponent(objects)} ${this.directory.name}`
    );

    assert.notOk(FS.breadcrumbs[0].isActive);

    assert.equal(FS.breadcrumbs[1].text, this.directory.name);
    assert.ok(FS.breadcrumbs[1].isActive);

    await FS.directoryEntries[0].visit();

    assert.equal(FS.directoryEntries.length, 1);
    assert.notOk(
      FS.directoryEntries[0].path.includes('//'),
      'paths shouldn’t have redundant separators'
    );

    assert.equal(FS.breadcrumbs.length, 3);
    assert.equal(
      FS.breadcrumbsText,
      `${getBreadcrumbComponent(objects)} ${this.directory.name} ${
        this.nestedDirectory.name
      }`
    );
    assert.equal(FS.breadcrumbs[2].text, this.nestedDirectory.name);

    assert.notOk(
      FS.breadcrumbs[0].path.includes('//'),
      'paths shouldn’t have redundant separators'
    );
    assert.notOk(
      FS.breadcrumbs[1].path.includes('//'),
      'paths shouldn’t have redundant separators'
    );

    await FS.breadcrumbs[1].visit();
    assert.equal(
      FS.breadcrumbsText,
      `${getBreadcrumbComponent(objects)} ${this.directory.name}`
    );
    assert.equal(FS.breadcrumbs.length, 2);
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
      'expected files to be sorted by descending size and directories to be sorted by descending name'
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
      'expected directories to be sorted by name and files to be sorted by ascending size'
    );
  });

  test('viewing a file', async function (assert) {
    const objects = { allocation: this.allocation, task: this.task };
    const node = server.db.nodes.find(this.allocation.nodeId);

    server.get(
      `http://${node.httpAddr}/v1/client/fs/readat/:allocation_id`,
      function () {
        return new Response(500);
      }
    );

    await FS[pageObjectVisitPathFunctionName]({
      ...visitSegments(objects),
      path: '/',
    });

    const sortedFiles = fileSort(
      'name',
      filesForPath(this.server.schema.allocFiles, getFilesystemRoot(objects))
        .models
    );
    const fileRecord = sortedFiles.find((f) => !f.isDir);
    const fileIndex = sortedFiles.indexOf(fileRecord);

    await FS.directoryEntries[fileIndex].visit();

    assert.equal(
      FS.breadcrumbsText,
      `${getBreadcrumbComponent(objects)} ${fileRecord.name}`
    );

    assert.ok(FS.fileViewer.isPresent);

    const requests = this.server.pretender.handledRequests;
    const secondAttempt = requests.pop();
    const firstAttempt = requests.pop();

    assert.equal(
      firstAttempt.url.split('?')[0],
      `//${node.httpAddr}/v1/client/fs/readat/${this.allocation.id}`,
      'Client is hit first'
    );
    assert.equal(firstAttempt.status, 500, 'Client request fails');
    assert.equal(
      secondAttempt.url.split('?')[0],
      `/v1/client/fs/readat/${this.allocation.id}`,
      'Server is hit second'
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
      '500 is not interpreted as 404'
    );
    assert.equal(
      FS.error.title,
      'Server Error',
      '500 is not interpreted as 500'
    );

    await visit('/');

    this.server.get('/client/fs/stat/:allocation_id', () => {
      return new Response(999);
    });

    await FS[pageObjectVisitPathFunctionName]({
      ...visitSegments({ allocation: this.allocation, task: this.task }),
      path: '/what-is-this',
    });
    assert.equal(FS.error.title, 'Error', 'other statuses are passed through');
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
      '500 is not interpreted as 404'
    );
    assert.equal(
      FS.error.title,
      'Server Error',
      '500 is not interpreted as 404'
    );

    await visit('/');

    this.server.get('/client/fs/ls/:allocation_id', () => {
      return new Response(999);
    });

    await FS[pageObjectVisitPathFunctionName]({
      ...visitSegments({ allocation: this.allocation, task: this.task }),
      path: this.directory.name,
    });
    assert.equal(FS.error.title, 'Error', 'other statuses are passed through');
  });
}
