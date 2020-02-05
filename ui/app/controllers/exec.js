import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
import { Terminal } from 'xterm';

export default Controller.extend({
  system: service(),

  queryParams: ['allocation'],

  init() {
    this._super(...arguments);

    this.terminal = new Terminal();
    window.execTerminal = this.terminal; // FIXME tragique, for acceptance tests…?

    this.terminal.writeln('Select a task to start your session.');
  },

  actions: {
    setAllocationAndTask({ allocation, allocationSpecified, task_name }) {
      this.terminal.writeln('');

      if (!allocationSpecified) {
        this.terminal.writeln(
          'Multiple instances of this task are running. The allocation below was selected by random draw.'
        );
        this.terminal.writeln('');
      }

      this.terminal.writeln(
        'To start the session, customize your command, then hit ‘return’ to run.'
      );
      this.terminal.writeln('');
      this.terminal.writeln(
        `$ nomad alloc exec -i -t -task ${task_name} ${allocation.shortId} /bin/bash`
      );
    },
  },
});
