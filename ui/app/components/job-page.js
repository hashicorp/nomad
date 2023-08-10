/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { action } from '@ember/object';
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import messageForError from 'nomad-ui/utils/message-from-adapter-error';

export default class JobPage extends Component {
  @tracked errorMessage = null;

  @action
  clearErrorMessage() {
    this.errorMessage = null;
  }

  @action
  handleError(errorObject) {
    this.errorMessage = errorObject;
  }

  @action
  setError(err) {
    this.errorMessage = {
      title: 'Could Not Force Launch',
      description: messageForError(err, 'submit jobs'),
    };
  }
}
