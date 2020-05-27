import { test } from 'qunit';
import { currentURL } from '@ember/test-helpers';
import FS from 'nomad-ui/tests/pages/allocations/task/fs';

export default function browseFilesystem({ pageObjectVisitPathFunctionName, visitSegments, getExpectedPathBase, getTitleComponent, getBreadcrumbComponent }) {
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
}
