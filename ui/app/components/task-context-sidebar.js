/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { inject as service } from '@ember/service';
import { task } from 'ember-concurrency';

export default class TaskContextSidebarComponent extends Component {
  @service notifications;
  @service nomadActions;

  get isSideBarOpen() {
    return !!this.args.task;
  }

  keyCommands = [
    {
      label: 'Close Task Logs Sidebar',
      pattern: ['Escape'],
      action: () => this.args.fns.closeSidebar(),
    },
  ];

  narrowCommand = {
    label: 'Narrow Sidebar',
    pattern: ['ArrowRight', 'ArrowRight'],
    action: () => this.toggleWide(),
  };

  widenCommand = {
    label: 'Widen Sidebar',
    pattern: ['ArrowLeft', 'ArrowLeft'],
    action: () => this.toggleWide(),
  };

  @tracked wide = false;
  @action toggleWide() {
    this.wide = !this.wide;
  }

  get shouldShowActions() {
    return (
      this.args.task.state === 'running' &&
      this.args.task.task.actions?.length &&
      this.nomadActions.hasActionPermissions
    );
  }

  @task(function* (action, allocID) {
    try {
      const job = this.args.task.task.taskGroup.job;
      yield job.runAction(action, allocID);
    } catch (err) {
      this.notifications.add({
        title: `Error starting ${action.name}`,
        message: err,
        color: 'critical',
      });
    }
  })
  runAction;
}
