/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Controller from '@ember/controller';
import { alias } from '@ember/object/computed';
// eslint-disable-next-line no-unused-vars
import VariableModel from '../../../models/variable';
// eslint-disable-next-line no-unused-vars
import JobModel from '../../../models/job';
// eslint-disable-next-line no-unused-vars
import MutableArray from '@ember/array/mutable';

export default class JobsJobVariablesController extends Controller {
  /** @type {JobModel} */
  @alias('model.job') job;

  /** @type {MutableArray<VariableModel>} */
  @alias('model.variables') variables;

  get firstFewTaskGroupNames() {
    return this.job.taskGroups.slice(0, 2).mapBy('name');
  }

  get firstFewTaskNames() {
    return this.job.taskGroups
      .map((tg) => tg.tasks.map((task) => `${tg.name}/${task.name}`))
      .flat()
      .slice(0, 2);
  }

  /**
   * Structures the flattened variables in a "path tree" like we use in the main variables routes
   * @returns {import("../../../utils/path-tree").VariableFolder}
   */
  get jobRelevantVariables() {
    /**
     * @type {import("../../../utils/path-tree").VariableFile[]}
     */
    let variableFiles = this.variables.map((v) => {
      return {
        name: v.path,
        path: v.path,
        absoluteFilePath: v.path,
        variable: v,
      };
    });

    return {
      files: variableFiles,
      children: {},
      absolutePath: '',
    };
  }
}
