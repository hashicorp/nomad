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

  @task(function* () {
    try {
      yield this.model.forcePause();
      this.notifications.add({
        title: 'Task Force Paused',
        message: 'Task has been force paused',
        color: 'success',
      });
    } catch (err) {
      this.set('error', {
        title: 'Could Not Force Pause Task',
      });
    }
  })
  forcePause;

  @task(function* () {
    try {
      yield this.model.forceRun();
      this.notifications.add({
        title: 'Task Force Run',
        message: 'Task has been force run',
        color: 'success',
      });
    } catch (err) {
      this.set('error', {
        title: 'Could Not Force Run Task',
      });
    }
  })
  forceRun;

  @task(function* () {
    try {
      yield this.model.reEnableSchedule();
      this.notifications.add({
        title: 'Task Put Back On Schedule',
        message: 'Task has been put back on its configured schedule',
        color: 'success',
      });
    } catch (err) {
      this.set('error', {
        title: 'Could Not put back on schedule',
      });
    }
  })
  reEnableSchedule;
}
