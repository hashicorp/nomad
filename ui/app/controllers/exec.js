/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
import { action, computed } from '@ember/object';
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
  @service store;
  @service token;

  queryParams = ['allocation', 'namespace'];
  fallbackAllocations = null;

  @localStorageProperty('nomadExecCommand', '/bin/bash') command;
  socketOpen = false;

  get pendingAndRunningAllocations() {
    return this.allocations.filter((allocation) => {
      const status = allocation.clientStatus || allocation.ClientStatus;
      return status === 'pending' || status === 'running';
    });
  }

  get allocations() {
    const relationshipAllocations = this.model?.allocations?.toArray?.() || [];
    if (relationshipAllocations.length) {
      return relationshipAllocations;
    }

    const jobCompositeId = this.model?.id;
    const jobPlainId = this.model?.plainId;
    const jobNamespace =
      this.model?.get?.('namespace.id') || this.namespace || 'default';
    const taskGroupNames = (this.model?.taskGroups || []).mapBy('name');

    const allocations =
      this.fallbackAllocations || this.store.peekAll('allocation');

    const byJob = allocations.filter((allocation) => {
      const allocationCompositeJobId = allocation.belongsTo('job').id();
      let allocPlainJobId = null;
      let allocNamespace = allocation.namespace || 'default';

      if (allocationCompositeJobId) {
        try {
          const [plainId, namespace] = JSON.parse(allocationCompositeJobId);
          allocPlainJobId = plainId;
          allocNamespace = namespace || allocNamespace;
        } catch {
          allocPlainJobId = null;
        }
      }

      const sameJob =
        (jobCompositeId && allocationCompositeJobId === jobCompositeId) ||
        (jobPlainId && allocPlainJobId === jobPlainId);

      return allocNamespace === jobNamespace && sameJob;
    });

    if (byJob.length) {
      return byJob;
    }

    const byTaskGroup = allocations.filter((allocation) => {
      const allocNamespace = allocation.namespace || 'default';
      return (
        allocNamespace === jobNamespace &&
        taskGroupNames.includes(allocation.taskGroupName)
      );
    });

    if (byTaskGroup.length) {
      return byTaskGroup;
    }

    // Last-resort fallback: preserve page usability when relationships are
    // missing by using in-namespace allocations.
    return allocations.filter(
      (allocation) => (allocation.namespace || 'default') === jobNamespace,
    );
  }

  get pendingAndRunningTaskGroups() {
    const allocations = this.pendingAndRunningAllocations || [];
    const taskGroups = this.model?.taskGroups || [];
    const names = [...new Set(allocations.map((alloc) => alloc.taskGroupName))]
      .filter(Boolean)
      .sort();

    return names
      .map((name) => {
        const hydratedTaskGroup = taskGroups.findBy('name', name);
        if (hydratedTaskGroup) {
          return hydratedTaskGroup;
        }

        const groupedAllocations = allocations.filterBy('taskGroupName', name);
        const groupedStates = groupedAllocations.flatMap(
          (allocation) =>
            allocation.states?.toArray?.() || allocation.states || [],
        );
        const taskNames = [
          ...new Set(groupedStates.map((state) => state?.name).filter(Boolean)),
        ];

        return {
          name,
          job: this.model,
          allocations: groupedAllocations,
          tasks: taskNames.map((taskName) => ({ name: taskName })),
        };
      })
      .filter(Boolean);
  }

  get sortedTaskGroups() {
    return [...(this.pendingAndRunningTaskGroups || [])].sort((a, b) =>
      a.name.localeCompare(b.name),
    );
  }

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

  @computed(
    'allocations.{[],@each.isActive}',
    'allocationShortId',
    'taskName',
    'taskGroupName',
    'allocation',
    'allocation.states.@each.{name,isRunning}',
  )
  get taskState() {
    if (!this.allocations) {
      return false;
    }

    let allocation;

    if (this.allocationShortId) {
      allocation = this.allocations.findBy('shortId', this.allocationShortId);
    } else {
      let allocationPool = this.taskGroupName
        ? this.allocations.filterBy('taskGroupName', this.taskGroupName)
        : this.allocations;
      allocation = allocationPool.find((allocation) =>
        allocation.states
          .filterBy('isActive')
          .mapBy('name')
          .includes(this.taskName),
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
          'Multiple instances of this task are running. The allocation below was selected by random draw.',
        );
        this.terminal.writeln('');
      }

      this.terminal.writeln(
        'Customize your command, then hit ‘return’ to run.',
      );
      this.terminal.writeln('');

      let namespaceCommandString = '';
      if (this.namespace && this.namespace !== 'default') {
        namespaceCommandString = `-namespace ${this.namespace} `;
      }

      this.terminal.write(
        `$ nomad alloc exec -i -t ${namespaceCommandString}-task ${escapeTaskName(
          taskName,
        )} ${this.taskState.allocation.shortId} `,
      );

      this.terminal.write(ANSI_WHITE);

      this.terminal.write(this.command);

      if (this.commandEditorAdapter) {
        this.commandEditorAdapter.destroy();
      }

      this.commandEditorAdapter = new ExecCommandEditorXtermAdapter(
        this.terminal,
        this.openAndConnectSocket.bind(this),
        this.command,
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
        `Failed to open a socket because task ${this.taskName} is not active.`,
      );
    }
  }
}
