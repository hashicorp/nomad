/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import compactPath from 'nomad-ui/utils/compact-path';
import pathTree from 'nomad-ui/utils/path-tree';
import { module, test } from 'qunit';

const PATHSTRINGS = [
  { path: 'a/b/c/d/e/foo0' },
  { path: 'a/b/c/d/e/bar1' },
  { path: 'a/b/c/d/e/baz2' },
  { path: 'a/b/c/d/e/z/z/z/z/z/z/z/z/z/z/foo3' },
  { path: 'z/y/x/dalmation/index' },
  { path: 'z/y/x/doberman/index' },
  { path: 'z/y/x/dachshund/index' },
  { path: 'z/y/x/dachshund/poppy' },
];

module('Unit | Utility | compact-path', function () {
  test('it compacts empty folders correctly', function (assert) {
    const tree = new pathTree(PATHSTRINGS);
    assert.ok(
      'a' in tree.paths.root.children,
      'root.a exists in the path tree despite having no files and only a single path'
    );
    assert.equal(
      compactPath(tree.root.children['a'], 'a').name,
      'a/b/c/d/e',
      'but root.a is displayed compacted down to /e from its root level folder'
    );
    assert.equal(
      compactPath(tree.findPath('z/y'), 'y').name,
      'y/x',
      'Path z/y is compacted to y/x, since it has a single child'
    );
    assert.equal(
      compactPath(tree.findPath('z/y/x'), 'x').name,
      'x',
      'Path z/y/x is uncompacted, since it has multiple children'
    );
    assert.equal(
      compactPath(tree.findPath('a/b/c/d/e/z'), 'z').name,
      'z/z/z/z/z/z/z/z/z/z',
      'Long path is recursively compacted'
    );
  });
});
