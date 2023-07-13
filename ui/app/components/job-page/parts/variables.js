/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@glimmer/component';
// import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';

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
  // #endregion Variables
}
