/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { computed as overridable } from 'ember-overridable-computed';
import { task } from 'ember-concurrency';
import classic from 'ember-classic-decorator';
import messageForError from 'nomad-ui/utils/message-from-adapter-error';

@classic
export default class IndexController extends Controller {
  @overridable(() => {
    // { title, description }
    return null;
  })
  error;

  onDismiss() {
    this.set('error', null);
  }

  @task(function* () {
    try {
      yield this.model.restart();
    } catch (err) {
      this.set('error', {
        title: 'Could Not Restart Task',
        description: messageForError(err, 'manage allocation lifecycle'),
      });
    }
  })
  restartTask;
}
