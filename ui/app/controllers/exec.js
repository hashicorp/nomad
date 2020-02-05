import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
import { Terminal } from 'xterm';

export default Controller.extend({
  sockets: service(),
  system: service(),

  queryParams: ['allocation'],

  init() {
    this._super(...arguments);

    // FIXME this hardcoding of a font is questionable, but can the monospace be determined from the CSS font stack? 🤔
    this.terminal = new Terminal({ fontFamily: 'SF Mono', fontWeight: '400' });
    window.execTerminal = this.terminal; // FIXME tragique, for acceptance tests…?

    // Sets the foreground colour to Structure’s ui-gray-400
    this.terminal.write('\x1b[38;2;142;150;163m');
    this.terminal.writeln('Select a task to start your session.');
  },

  actions: {
    setTaskState({ allocationSpecified, taskState }) {
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
      this.terminal.write(
        `$ nomad alloc exec -i -t -task ${taskState.name} ${taskState.allocation.shortId} `
      );
      // FIXME task names might need quotes…?

      // Sets the foreground colour to white
      this.terminal.write('\x1b[0m');

      this.terminal.write('/bin/bash');

      let socketOpen = false;

      this.terminal.onKey(e => {
        if (e.domEvent.key === 'Enter' && !socketOpen) {
          this.openAndConnectSocket(taskState);
          this.terminal.writeln('');
          socketOpen = true;
        } else {
          this.socket.send(JSON.stringify({ stdin: { data: btoa(e.key) } }));
        }
      });
    },
  },

  openAndConnectSocket(taskState) {
    this.socket = this.sockets.getTaskStateSocket(taskState);

    this.socket.onmessage = e => {
      const json = JSON.parse(e.data);
      this.terminal.write(atob(json.stdout.data));
    };
  },
});
