/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { computed as overridable } from 'ember-overridable-computed';
import { task } from 'ember-concurrency';
import classic from 'ember-classic-decorator';
import messageForError from 'nomad-ui/utils/message-from-adapter-error';
import { inject as service } from '@ember/service';

@classic
export default class IndexController extends Controller {
  @service nomadActions;
  @service notifications;
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

  get shouldShowActions() {
    return (
      this.model.state === 'running' &&
      this.model.task.actions?.length &&
      this.nomadActions.hasActionPermissions
    );
  }

  /**
   * @param {string} action - The action to run
   * @param {string} allocID - The allocation ID to run the action on
   * @param {Event} ev - The event that triggered the action
   */
  @task(function* (action, allocID) {
    // Yo this is such a dumb bug! allocID is getting the 0th entry in task.actions.allocations but those are populated regardless of whether we're in a task context (which would necessitate a specific allocID)
    try {
      const job = this.model.task.taskGroup.job;
      // TODO: have the service handle "all" vs specific
      yield this.nomadActions.runAction(action, allocID, job);
    } catch (err) {
      this.notifications.add({
        title: `Error starting ${action.name}`,
        message: err,
        sticky: true,
        color: 'critical',
      });
    }
  })
  runAction;
}
