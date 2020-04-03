import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { mapBy, sort, uniq } from '@ember/object/computed';
import escapeTaskName from 'nomad-ui/utils/escape-task-name';
import ExecCommandEditorXtermAdapter from 'nomad-ui/utils/classes/exec-command-editor-xterm-adapter';
import ExecSocketXtermAdapter from 'nomad-ui/utils/classes/exec-socket-xterm-adapter';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

import { Terminal } from 'xterm-vendor';

const ANSI_UI_GRAY_400 = '\x1b[38;2;142;150;163m';
const ANSI_WHITE = '\x1b[0m';

export default Controller.extend({
  sockets: service(),
  system: service(),

  queryParams: ['allocation'],

  command: localStorageProperty('nomadExecCommand', '/bin/bash'),
  socketOpen: false,
  taskState: null,

  pendingAndRunningAllocations: computed('model.allocations.@each.clientStatus', function() {
    return this.model.allocations.filter(
      allocation => allocation.clientStatus === 'pending' || allocation.clientStatus === 'running'
    );
  }),

  pendingAndRunningTaskGroups: mapBy('pendingAndRunningAllocations', 'taskGroup'),
  uniquePendingAndRunningTaskGroups: uniq('pendingAndRunningTaskGroups'),

  taskGroupSorting: Object.freeze(['name']),
  sortedTaskGroups: sort('uniquePendingAndRunningTaskGroups', 'taskGroupSorting'),

  init() {
    this._super(...arguments);

    this.terminal = new Terminal({ fontFamily: 'monospace', fontWeight: '400' });
    window.execTerminal = this.terminal; // Issue to improve: https://github.com/hashicorp/nomad/issues/7457

    this.terminal.write(ANSI_UI_GRAY_400);
    this.terminal.writeln('Select a task to start your session.');
  },

  actions: {
    setTaskState({ allocationSpecified, taskState }) {
      this.set('taskState', taskState);

      this.terminal.write(ANSI_UI_GRAY_400);
      this.terminal.writeln('');

      if (!allocationSpecified) {
        this.terminal.writeln(
          'Multiple instances of this task are running. The allocation below was selected by random draw.'
        );
        this.terminal.writeln('');
      }

      this.terminal.writeln('Customize your command, then hit ‘return’ to run.');
      this.terminal.writeln('');
      this.terminal.write(
        `$ nomad alloc exec -i -t -task ${escapeTaskName(taskState.name)} ${
          taskState.allocation.shortId
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
    },
  },

  openAndConnectSocket(command) {
    this.set('socketOpen', true);
    this.set('command', command);
    this.socket = this.sockets.getTaskStateSocket(this.taskState, command);

    new ExecSocketXtermAdapter(this.terminal, this.socket);
  },
});
