import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
import { Terminal } from 'xterm';

export default Controller.extend({
  system: service(),

  init() {
    this._super(...arguments);

    this.terminal = new Terminal();
    window.execTerminal = this.terminal; // FIXME tragique, for acceptance tests…?

    this.terminal.writeln('Select a task to start your session.');
  },

  actions: {
    setTask({ task_name }) {
      this.terminal.writeln('');
      this.terminal.writeln(
        'To start the session, customize your command, then hit ‘return’ to run.'
      );
      this.terminal.writeln('');
      this.terminal.writeln(`$ nomad alloc exec -i -t -task ${task_name} ALLOCATION /bin/bash`);
    },
  },
});
