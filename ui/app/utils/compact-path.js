/**
 * Takes a branch created by our path-tree, and if it has only a single directory as descendent and no files, compacts it down to its terminal folder (the first descendent folder with either files or branching directories)
 * Uses tail recursion
 * @param {import("./path-tree").NestedPathTreeNode} branch
 * @returns
 */
export default function compactPath(branch, name = '') {
  // console.log('lup', branch);
  // name = `${name}/${branch.name}`;
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

// if (branch.children && Object.keys(branch.children).length === 1) {
//   const [name, child] = Object.entries(branch.children)[0];
//   console.log('true and', name, child, child.children, !Object.keys(child.children).length);
//   if (child.files.length && !Object.keys(child.children).length) {
//     console.log('also true!');
//     return {
//       name,
//       absolutePath: branch.absolutePath,
//       files: child.files,
//     };
//   }
// }
// return branch;
