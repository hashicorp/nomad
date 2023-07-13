/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// @ts-check

import Component from '@glimmer/component';
// import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import VariableModel from '../../../models/variable';
import MutableArray from '@ember/array/mutable';

export default class Variables extends Component {
  // @service system;

  @alias('args.job') job;
  // constructor(a,b,c) {
  //   super(...arguments);
  //   console.log('Variables constructor', this.args.job, this.args.job.pathLinkedVariable, this.args.job.get('pathLinkedVariable'));
  // }

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
      this.job.pathLinkedVariable,
      ...this.job.taskGroups.mapBy('pathLinkedVariable'),
      ...this.job.taskGroups
        .map((tg) => tg.tasks.mapBy('pathLinkedVariable'))
        .flat(),
    ].compact();

    console.log('flatvars, then', flatVariables);

    /**
     * @type {import("../../../utils/path-tree").VariableFile[]}
     */
    let variableFiles = flatVariables.map((v) => {
      console.log('vee', v);
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
