import { test } from 'qunit';
import { currentURL, visit } from '@ember/test-helpers';
import FS from 'nomad-ui/tests/pages/allocations/task/fs';
import moment from 'moment';
import { filesForPath } from 'nomad-ui/mirage/config';
import Response from 'ember-cli-mirage/response';

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

export default function browseFilesystem({ pageObjectVisitPathFunctionName, visitSegments, getExpectedPathBase, getTitleComponent, getBreadcrumbComponent, getFilesystemRoot }) {
  test('visiting filesystem paths', async function(assert) {
    const paths = ['some-file.log', 'a/deep/path/to/a/file.log', '/', 'Unicode™®'];

    const testPath = async filePath => {
      let pathWithLeadingSlash = filePath;

      if (!pathWithLeadingSlash.startsWith('/')) {
        pathWithLeadingSlash = `/${filePath}`;
      }

      await FS[pageObjectVisitPathFunctionName]({ ...visitSegments({allocation: this.allocation, task: this.task }), path: filePath });
      assert.equal(
        currentURL(),
        `${getExpectedPathBase({allocation: this.allocation, task: this.task })}${encodeURIComponent(filePath)}`,
        'No redirect'
      );
      assert.equal(
        document.title,
        `${pathWithLeadingSlash} - ${getTitleComponent({allocation: this.allocation, task: this.task})} - Nomad`
      );
      assert.equal(FS.breadcrumbsText, `${getBreadcrumbComponent({allocation: this.allocation, task: this.task})} ${filePath.replace(/\//g, ' ')}`.trim());
    };

    await paths.reduce(async (prev, filePath) => {
      await prev;
      return testPath(filePath);
    }, Promise.resolve());
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

    await FS[pageObjectVisitPathFunctionName]({ ...visitSegments({allocation: this.allocation, task: this.task }), path: '/' });

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

  test('viewing a file', async function(assert) {
    const objects = { allocation: this.allocation, task: this.task };
    const node = server.db.nodes.find(this.allocation.nodeId);

    server.get(`http://${node.httpAddr}/v1/client/fs/readat/:allocation_id`, function() {
      return new Response(500);
    });

    await FS[pageObjectVisitPathFunctionName]({ ...visitSegments(objects), path: '/' });

    const sortedFiles = fileSort('name', filesForPath(this.server.schema.allocFiles, getFilesystemRoot(objects)).models);
    const fileRecord = sortedFiles.find(f => !f.isDir);
    const fileIndex = sortedFiles.indexOf(fileRecord);

    await FS.directoryEntries[fileIndex].visit();

    assert.equal(FS.breadcrumbsText, `${getBreadcrumbComponent(objects)} ${fileRecord.name}`);

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

  test('viewing an empty directory', async function(assert) {
    await FS[pageObjectVisitPathFunctionName]({ ...visitSegments({ allocation: this.allocation, task: this.task }), path: 'empty-directory' });

    assert.ok(FS.isEmptyDirectory);
  });

  test('viewing paths that produce stat API errors', async function(assert) {
    this.server.get('/client/fs/stat/:allocation_id', () => {
      return new Response(500, {}, 'no such file or directory');
    });

    await FS[pageObjectVisitPathFunctionName]({ ...visitSegments({ allocation: this.allocation, task: this.task }), path: '/what-is-this' });
    assert.equal(FS.error.title, 'Not Found', '500 is interpreted as 404');

    await visit('/');

    this.server.get('/client/fs/stat/:allocation_id', () => {
      return new Response(999);
    });

    await FS[pageObjectVisitPathFunctionName]({ ...visitSegments({ allocation: this.allocation, task: this.task }), path: '/what-is-this' });
    assert.equal(FS.error.title, 'Error', 'other statuses are passed through');
  });

  test('viewing paths that produce ls API errors', async function(assert) {
    this.server.get('/client/fs/ls/:allocation_id', () => {
      return new Response(500, {}, 'no such file or directory');
    });

    await FS[pageObjectVisitPathFunctionName]({ ...visitSegments({ allocation: this.allocation, task: this.task }), path: this.directory.name });
    assert.equal(FS.error.title, 'Not Found', '500 is interpreted as 404');

    await visit('/');

    this.server.get('/client/fs/ls/:allocation_id', () => {
      return new Response(999);
    });

    await FS[pageObjectVisitPathFunctionName]({ ...visitSegments({ allocation: this.allocation, task: this.task }), path: this.directory.name });
    assert.equal(FS.error.title, 'Error', 'other statuses are passed through');
  });
}
