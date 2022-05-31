// @ts-check

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { alias, reads } from '@ember/object/computed';
import VariableModel from '../../models/variable';
import MutableArray from '@ember/array/mutable';
import { trimPath } from '../../helpers/trim-path';

//#region Types
/**
 * @typedef {Object} VariablePathObject
 * @property {string} path - the folder path containing our "file"
 * @property {string} file - the secure variable "file" name
 */

/**
 * @typedef {Object.<string, Object>} NestedPathTreeNode
 */
//#endregion Types

/**
 * Turns a file path into an object with file and path properties.
 * @param {string} path - the file path
 * @return {VariablePath Object}
 */
function PATH_TO_OBJECT(path) {
  const split = path.split('/');
  const [file, ...folderPath] = [split.pop(), ...split];
  return {
    file,
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

export default class VariablesIndexController extends Controller {
  @service router;

  /**
   * @type {MutableArray<VariableModel>}
   */
  @alias('model.variables') allVariables;

  isForbidden = false;

  /**
   * Takes our variables array and groups them by common path
   * @returns {NestedPathTreeNode}
   */
  get variablePathTree() {
    const paths = this.allVariables
      .map((variable) => trimPath([variable.path]))
      .map(PATH_TO_OBJECT)
      .reduce(
        (acc, cur) => {
          if (cur.path) {
            acc.children[cur.path]
              ? acc.children[cur.path].files.push(cur.file)
              : (acc.children[cur.path] = { files: [cur.file], children: {} });
          } else {
            acc.files ? acc.files.push(cur.file) : (acc.files = [cur.file]);
          }
          return acc;
        },
        { files: [], children: {} }
      );

    COMPACT_EMPTY_DIRS(paths.children);
    console.log({ paths });
    return paths;
  }

  @action
  goToVariable(variable) {
    this.router.transitionTo('variables.variable', variable.path);
  }
}
