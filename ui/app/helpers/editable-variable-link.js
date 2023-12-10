/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
// eslint-disable-next-line no-unused-vars
import VariableModel from '../models/variable';
// eslint-disable-next-line no-unused-vars
import MutableArray from '@ember/array/mutable';

/**
 * @typedef LinkToParams
 * @property {string} route
 * @property {string} model
 * @property {Object} query
 */

import Helper from '@ember/component/helper';

/**
 * Either generates a link to edit an existing variable, or else create a new one with a pre-filled path, depending on whether a variable with the given path already exists.
 * Returns an object with route, model, and query; all strings.
 * @param {Array<string>} positional
 * @param {{ existingPaths: MutableArray<VariableModel>, namespace: string }} named
 * @returns {LinkToParams}
 */
export function editableVariableLink(
  [path],
  { existingPaths, namespace = 'default' }
) {
  if (existingPaths.findBy('path', path)) {
    return {
      route: 'variables.variable.edit',
      model: `${path}@${namespace}`,
      query: {},
    };
  } else {
    return {
      route: 'variables.new',
      model: '',
      query: { path },
    };
  }
}

export default Helper.helper(editableVariableLink);
