/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

// eslint-disable-next-line no-unused-vars
import VariableModel from '../models/variable';
// eslint-disable-next-line no-unused-vars
import MutableArray from '@ember/array/mutable';
import { trimPath } from '../helpers/trim-path';

//#region Types
/**
 * @typedef {Object} VariableFile
 * @property {string} path - the folder path containing our "file", relative to parent
 * @property {string} name - the variable "file" name
 * @property {string} absoluteFilePath - the folder path containing our "file", absolute
 * @property {VariableModel} variable - the variable itself
 */

/**
 * @typedef {Object} VariableFolder
 * @property {Array<VariableFile>} files
 * @property {NestedPathTreeNode} children
 * @property {string} absolutePath - the folder path containing our "file", absolute
 */

/**
 * @typedef {Object.<string, VariableFolder>} NestedPathTreeNode
 */
//#endregion Types

/**
 * @returns {NestedPathTreeNode}
 */
export default class PathTree {
  /**
   * @param {MutableArray<VariableModel>} variables
   */
  constructor(variables) {
    this.variables = variables;
    this.paths = this.generatePaths(variables);
  }

  /**
   * @type {VariableFolder}
   */
  root = { children: {}, files: [], absolutePath: '' };

  /**
   * Takes our variables array and creates a tree of paths
   * @param {MutableArray<VariableModel>} variables
   * @returns {NestedPathTreeNode}
   */
  generatePaths = (variables) => {
    variables.forEach((variable) => {
      const path = trimPath([variable.path]).split('/');
      path.reduce((acc, segment, index, arr) => {
        if (index === arr.length - 1) {
          // If it's a file (end of the segment array)
          acc.files.push({
            name: segment,
            absoluteFilePath: path.join('/'),
            path: arr.slice(0, index + 1).join('/'),
            variable,
          });
        } else {
          // Otherwise, it's a folder
          if (!acc.children[segment]) {
            acc.children[segment] = {
              children: {},
              files: [],
              absolutePath: trimPath([`${acc.absolutePath || ''}/${segment}`]),
            };
          }
        }
        return acc.children[segment];
      }, this.root);
    });
    return { root: this.root };
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
