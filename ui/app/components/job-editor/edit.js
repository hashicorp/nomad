/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class Edit extends Component {
  @tracked updateHclVariablesEditor = null;
  @tracked jobDefinitionEditor = null;

  @action
  updateHclVariables(value) {
    this.args.fns.onUpdate(
      value,
      this.updateHclVariablesEditor,
      'hclVariables'
    );
  }

  @action
  updateJobDefinition(value) {
    this.args.fns.onUpdate(value, this.jobDefinitionEditor, 'job');
  }
}
