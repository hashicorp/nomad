/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// @ts-check

import Controller from '@ember/controller';
import { alias } from '@ember/object/computed';
import VariableModel from '../../../models/variable';
import MutableArray from '@ember/array/mutable';

export default class JobsJobVariablesController extends Controller {
  @alias('model.job') job;
  // #region Variables
  get hasJobLevelVariables() {
    return !!this.job.pathLinkedVariable;
  }

  get hasGroupLevelVariables() {
    return this.job.taskGroups.any((tg) => tg.pathLinkedVariable);
  }

  get hasTaskLevelVariables() {
    return this.job.taskGroups.any((tg) =>
      tg.tasks.any((task) => task.pathLinkedVariable)
    );
  }

  /**
   * @returns {import("../../../utils/path-tree").VariableFolder}
   */
  get jobRelevantVariables() {
    /**
     * @type {MutableArray<VariableModel>}
     */
    let flatVariables = [
      this.model.variables.findBy('path', 'nomad/jobs'),
      this.job.pathLinkedVariable,
      ...this.job.taskGroups.mapBy('pathLinkedVariable'),
      ...this.job.taskGroups
        .map((tg) => tg.tasks.mapBy('pathLinkedVariable'))
        .flat(),
    ].compact();

    /**
     * @type {import("../../../utils/path-tree").VariableFile[]}
     */
    let variableFiles = flatVariables.map((v) => {
      return {
        name: v.path, // TODO: check if this is right or if we just want the last post-/ segment
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
  // #endregion Variables
}
