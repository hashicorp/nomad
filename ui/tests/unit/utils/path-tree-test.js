import pathTree from 'nomad-ui/utils/path-tree';
import { module, test } from 'qunit';

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

module('Unit | Utility | path-tree', function () {
  test('it converts path strings to a Variable Path Object ', function (assert) {
    const tree = new pathTree(PATHSTRINGS);
    assert.ok(
      'root' in tree.paths,
      'Tree has a paths object that begins with a root'
    );
    assert.ok('children' in tree.paths.root, 'Root has children');
    assert.ok(
      Object.keys(tree.paths.root.children).length === 2,
      'Root has 2 children (a[...] and foo[...])'
    );
  });

  test('it compacts empty folders correctly', function (assert) {
    const tree = new pathTree(PATHSTRINGS);
    assert.ok(
      'a' in tree.paths.root.children,
      'root.a is uncompacted since it contains a file (b)'
    );
    assert.notOk(
      'foo' in tree.paths.root.children,
      'root.foo does not exist since it contains no files'
    );
    assert.ok(
      'foo/bar' in tree.paths.root.children,
      'root.foo/bar is compacted since the only child from foo is bar'
    );
    assert.equal(
      tree.paths.root.children['foo/bar'].files.length,
      3,
      'A compacted directory contains all terminal files'
    );
  });

  test('it allows for node-based search and traversal', function (assert) {
    const tree = new pathTree(PATHSTRINGS);
    const foundPath = tree.findPath('a/b');
    assert.deepEqual(
      tree.paths.root,
      tree.findPath(''),
      'Returns tree root on default findPath'
    );
    assert.notOk(
      tree.findPath('foo'),
      'No path found at the first part of a concatenated folder'
    ); // TODO: but maybe we want this to work eventually, so if this test fails because you add mid-tree traversal? Great!
    assert.ok(
      tree.findPath('foo/bar'),
      'Finds a path at the concatenated folder path'
    );
    assert.ok(
      tree.findPath('a/b'),
      'Finds a path at the concatenated folder path with multiple subdirectories'
    );

    assert.equal(
      Object.keys(tree.findPath('a/b/c').children).length,
      3,
      'Multiple subdirectories are listed at a found compacted path with many child paths'
    );

    assert.equal(
      Object.keys(tree.findPath('a/b').files).length,
      4,
      'Multiple files are listed at a found non-terminal compacted path with many secure variables'
    );
    assert.equal(
      Object.keys(tree.findPath('a/b/c/doberman').files).length,
      1,
      'One file listed at a found compacted path with a single secure variable'
    );
    assert.equal(
      Object.keys(tree.findPath('a/b/c/dachshund').files).length,
      2,
      'Multiple files listed at a found terminal compacted path with many secure variables'
    );
  });
});
