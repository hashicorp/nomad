/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/**
 * Takes a branch created by our path-tree, and if it has only a single directory as descendent and no files, compacts it down to its terminal folder (the first descendent folder with either files or branching directories)
 * Uses tail recursion
 * @param {import("./path-tree").NestedPathTreeNode} branch
 * @returns
 */
export default function compactPath(branch, name = '') {
  let { children, files } = branch;
  if (children && Object.keys(children).length === 1 && !files.length) {
    const [key] = Object.keys(children);
    const child = children[key];
    return compactPath(child, `${name}/${key}`);
  }
  return {
    name,
    data: branch,
  };
}
