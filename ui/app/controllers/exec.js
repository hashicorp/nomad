/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
import { action, computed } from '@ember/object';
import { alias, mapBy, sort, uniq } from '@ember/object/computed';
import escapeTaskName from 'nomad-ui/utils/escape-task-name';
import ExecCommandEditorXtermAdapter from 'nomad-ui/utils/classes/exec-command-editor-xterm-adapter';
import ExecSocketXtermAdapter from 'nomad-ui/utils/classes/exec-socket-xterm-adapter';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';
import classic from 'ember-classic-decorator';

const ANSI_UI_GRAY_400 = '\x1b[38;2;142;150;163m';
const ANSI_WHITE = '\x1b[0m';

@classic
export default class ExecController extends Controller {
  @service sockets;
  @service system;
  @service token;

  queryParams = ['allocation', 'namespace'];

  @localStorageProperty('nomadExecCommand', '/bin/bash') command;
  socketOpen = false;

  @computed('model.allocations.@each.clientStatus')
  get pendingAndRunningAllocations() {
    return this.model.allocations.filter(
      (allocation) =>
        allocation.clientStatus === 'pending' ||
        allocation.clientStatus === 'running'
    );
  }

  @mapBy('pendingAndRunningAllocations', 'taskGroup')
  pendingAndRunningTaskGroups;
  @uniq('pendingAndRunningTaskGroups') uniquePendingAndRunningTaskGroups;

  taskGroupSorting = ['name'];
  @sort('uniquePendingAndRunningTaskGroups', 'taskGroupSorting')
  sortedTaskGroups;

  setUpTerminal(Terminal) {
    this.terminal = new Terminal({
      fontFamily: 'monospace',
      fontWeight: '400',
    });
    window.execTerminal = this.terminal; // Issue to improve: https://github.com/hashicorp/nomad/issues/7457

    this.terminal.write(ANSI_UI_GRAY_400);

    if (this.sortedTaskGroups.length > 0) {
      this.terminal.writeln('Select a task to start your session.');
    } else {
      this.terminal.writeln(`There are no tasks running for this job.`);
    }
  }

  @alias('model.allocations') allocations;

  @computed(
    'allocations.{[],@each.isActive}',
    'allocationShortId',
    'taskName',
    'taskGroupName',
    'allocation',
    'allocation.states.@each.{name,isRunning}'
  )
  get taskState() {
    if (!this.allocations) {
      return false;
    }

    let allocation;

    if (this.allocationShortId) {
      allocation = this.allocations.findBy('shortId', this.allocationShortId);
    } else {
      allocation = this.allocations.find((allocation) =>
        allocation.states
          .filterBy('isActive')
          .mapBy('name')
          .includes(this.taskName)
      );
    }

    if (allocation) {
      return allocation.states.find((state) => state.name === this.taskName);
    }

    return undefined;
  }

  @action
  setTaskProperties({ allocationShortId, taskName, taskGroupName }) {
    this.setProperties({
      allocationShortId,
      taskName,
      taskGroupName,
    });

    if (this.taskState) {
      this.terminal.write(ANSI_UI_GRAY_400);
      this.terminal.writeln('');

      if (!allocationShortId) {
        this.terminal.writeln(
          'Multiple instances of this task are running. The allocation below was selected by random draw.'
        );
        this.terminal.writeln('');
      }

      this.terminal.writeln(
        'Customize your command, then hit ‘return’ to run.'
      );
      this.terminal.writeln('');
      this.terminal.write(
        `$ nomad alloc exec -i -t -task ${escapeTaskName(taskName)} ${
          this.taskState.allocation.shortId
        } `
      );

      this.terminal.write(ANSI_WHITE);

      this.terminal.write(this.command);

      if (this.commandEditorAdapter) {
        this.commandEditorAdapter.destroy();
      }

      this.commandEditorAdapter = new ExecCommandEditorXtermAdapter(
        this.terminal,
        this.openAndConnectSocket.bind(this),
        this.command
      );
    }
  }

  openAndConnectSocket(command) {
    if (this.taskState) {
      this.set('socketOpen', true);
      this.set('command', command);
      this.socket = this.sockets.getTaskStateSocket(this.taskState, command);

      new ExecSocketXtermAdapter(this.terminal, this.socket, this.token.secret);
    } else {
      this.terminal.writeln(
        `Failed to open a socket because task ${this.taskName} is not active.`
      );
    }
  }
}
