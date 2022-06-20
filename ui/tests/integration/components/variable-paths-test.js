import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import pathTree from 'nomad-ui/utils/path-tree';

const PATHSTRINGS = [
  { path: '/foo/bar/baz' },
  { path: '/foo/bar/bay' },
  { path: '/foo/bar/bax' },
  { path: '/a/b' },
  { path: '/a/b/c' },
  { path: '/a/b/canary' },
  { path: '/a/b/canine' },
  { path: '/a/b/chipmunk' },
  { path: '/a/b/c/d' },
  { path: '/a/b/c/dalmation/index' },
  { path: '/a/b/c/doberman/index' },
  { path: '/a/b/c/dachshund/index' },
  { path: '/a/b/c/dachshund/poppy' },
];
const tree = new pathTree(PATHSTRINGS);

module('Integration | Component | variable-paths', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders without data', async function (assert) {
    assert.expect(2);

    this.set('emptyRoot', { children: {}, files: [] });
    await render(hbs`<VariablePaths @branch={{this.emptyRoot}} />`);
    assert.dom('tbody tr').exists({ count: 0 });

    await componentA11yAudit(this.element, assert);
  });

  test('it renders with data', async function (assert) {
    assert.expect(2);

    this.set('tree', tree);
    await render(hbs`<VariablePaths @branch={{this.tree.paths.root}} />`);
    assert.dom('tbody tr').exists({ count: 2 }, 'There are two rows');

    await componentA11yAudit(this.element, assert);
  });

  test('it allows for traversal: Folders', async function (assert) {
    assert.expect(3);

    this.set('tree', tree);
    await render(hbs`<VariablePaths @branch={{this.tree.paths.root}} />`);
    assert
      .dom('tbody tr:first-child td:first-child a')
      .hasAttribute(
        'href',
        '/ui/variables/path/foo/bar',
        'Correctly links a folder'
      );
    assert
      .dom('tbody tr:first-child svg')
      .hasAttribute(
        'data-test-icon',
        'folder',
        'Correctly renders the folder icon'
      );

    await componentA11yAudit(this.element, assert);
  });

  test('it allows for traversal: Files', async function (assert) {
    assert.expect(5);

    this.set('tree', tree.findPath('foo/bar'));
    await render(hbs`<VariablePaths @branch={{this.tree}} />`);
    assert
      .dom('tbody tr:first-child td:first-child a')
      .hasAttribute(
        'href',
        '/ui/variables/var/foo/bar/baz',
        'Correctly links the first file'
      );
    assert
      .dom('tbody tr:nth-child(2) td:first-child a')
      .hasAttribute(
        'href',
        '/ui/variables/var/foo/bar/bay',
        'Correctly links the second file'
      );
    assert
      .dom('tbody tr:nth-child(3) td:first-child a')
      .hasAttribute(
        'href',
        '/ui/variables/var/foo/bar/bax',
        'Correctly links the third file'
      );
    assert
      .dom('tbody tr:first-child svg')
      .hasAttribute(
        'data-test-icon',
        'file-text',
        'Correctly renders the file icon'
      );
    await componentA11yAudit(this.element, assert);
  });
});
