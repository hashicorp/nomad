/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

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
    assert.equal(
      Object.keys(tree.paths.root.children).length,
      2,
      'Root has 2 children (a[...] and foo[...])'
    );
  });

  test('it allows for node-based search and traversal', function (assert) {
    const tree = new pathTree(PATHSTRINGS);
    assert.deepEqual(
      tree.paths.root,
      tree.findPath(''),
      'Returns tree root on default findPath'
    );
    assert.ok(
      tree.findPath('foo'),
      'Path found at the first part of a concatenated folder'
    );
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
      'Multiple files are listed at a found non-terminal compacted path with many variables'
    );
    assert.equal(
      Object.keys(tree.findPath('a/b/c/doberman').files).length,
      1,
      'One file listed at a found compacted path with a single variable'
    );
    assert.equal(
      Object.keys(tree.findPath('a/b/c/dachshund').files).length,
      2,
      'Multiple files listed at a found terminal compacted path with many variables'
    );
  });
});
