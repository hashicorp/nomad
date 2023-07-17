/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// @ts-check

import Controller from '@ember/controller';
import { alias } from '@ember/object/computed';
import VariableModel from '../../../models/variable';
import JobModel from '../../../models/job';
import MutableArray from '@ember/array/mutable';
import { A } from '@ember/array';

export default class JobsJobVariablesController extends Controller {
  /** @type {JobModel} */
  @alias('model.job') job;

  /** @type {MutableArray<VariableModel>} */
  @alias('model.variables') variables;

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

  get firstFewTaskGroupNames() {
    return this.job.taskGroups.slice(0, 3).mapBy('name');
  }

  get firstFewTaskNames() {
    return this.job.taskGroups
      .slice(0, 3)
      .map((tg) =>
        tg.tasks.slice(0, 3).map((task) => `${tg.name}/${task.name}`)
      )
      .flat();
  }

  /**
   * @returns {import("../../../utils/path-tree").VariableFolder}
   */
  get jobRelevantVariables() {
    /**
     * @type {MutableArray<VariableModel>}
     */
    let flatVariables = A([
      this.variables.findBy('path', 'nomad/jobs'),
      this.job.pathLinkedVariable,
      ...this.job.taskGroups.mapBy('pathLinkedVariable'),
      ...this.job.taskGroups
        .map((tg) => tg.tasks.mapBy('pathLinkedVariable'))
        .flat(),
    ]).compact();

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
}
