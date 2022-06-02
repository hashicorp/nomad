// @ts-check

// eslint-disable-next-line no-unused-vars
import VariableModel from '../models/variable';
// eslint-disable-next-line no-unused-vars
import MutableArray from '@ember/array/mutable';
import { trimPath } from '../helpers/trim-path';

//#region Types
/**
 * @typedef {Object} VariablePathObject
 * @property {string} path - the folder path containing our "file", relative to parent
 * @property {string} name - the secure variable "file" name
 * @property {string} [absoluteFilePath] - the folder path containing our "file", absolute
 * @property {string} [absolutePath] - the folder path containing our "file", absolute
 */

/**
 * @typedef {Object.<string, Object>} NestedPathTreeNode
 */
//#endregion Types

/**
 * Turns a file path into an object with file and path properties.
 * @param {string} path - the file path
 * @return {VariablePathObject}
 */
function PATH_TO_OBJECT(path) {
  const split = path.split('/');
  const [name, ...folderPath] = [split.pop(), ...split];
  return {
    name,
    absoluteFilePath: path,
    path: folderPath.join('/'),
  };
}

/**
 * Compacts an object of path:file key-values so any same-common-ancestor paths are collapsed into a single path.
 * @param {NestedPathTreeNode} vars
 * @returns {void}}
 */
function COMPACT_EMPTY_DIRS(vars) {
  Object.keys(vars).map((pathString) => {
    const encompasser = Object.keys(vars).find(
      (ps) => ps !== pathString && pathString.startsWith(ps)
    );
    if (encompasser) {
      vars[encompasser].children[pathString.replace(encompasser, '')] =
        vars[pathString];
      delete vars[pathString];
      COMPACT_EMPTY_DIRS(vars[encompasser].children);
    }
  });
}

/**
 * @returns {NestedPathTreeNode}
 */
export default class PathTree {
  /**
   * @param {MutableArray<VariableModel>} variables
   */
  constructor(variables) {
    this.variables = variables;
    this.paths = this.generatePaths();
  }

  /**
   * Takes our variables array and groups them by common path
   * @returns {NestedPathTreeNode}
   */
  generatePaths = () => {
    const paths = this.variables
      .map((variable) => trimPath([variable.path]))
      .map(PATH_TO_OBJECT)
      .reduce(
        (acc, cur) => {
          const { name, absoluteFilePath } = cur;
          if (cur.path) {
            acc.root.children[cur.path]
              ? acc.root.children[cur.path].files.push({
                  name,
                  absoluteFilePath,
                })
              : (acc.root.children[cur.path] = {
                  files: [{ name, absoluteFilePath }],
                  children: {},
                });
            acc.root.children[cur.path].absolutePath = cur.path;
          } else {
            acc.root.files
              ? acc.root.files.push({ name, absoluteFilePath })
              : (acc.root.files = [{ name, absoluteFilePath }]);
          }
          return acc;
        },
        { root: { files: [], children: {}, absolutePath: '' } }
      );

    COMPACT_EMPTY_DIRS(paths.root.children);
    return paths;
  };

  /**
   * Search for the named absolutePath within our tree using recursion
   * @param {string} name
   * @param {Object} root
   */
  findPath = (name, root = this.paths.root) => {
    if (root.absolutePath === name) {
      return root;
    }
    if (root.children) {
      return Object.keys(root.children).reduce((acc, cur) => {
        if (!acc) {
          return this.findPath(name, root.children[cur]);
        }
        return acc;
      }, null);
    }
  };
}
