import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
import { Terminal } from 'xterm';

export default Controller.extend({
  sockets: service(),
  system: service(),

  queryParams: ['allocation'],

  command: '/bin/bash',
  socketOpen: false,
  taskState: null,

  init() {
    this._super(...arguments);

    this.terminal = new Terminal({ fontFamily: 'monospace', fontWeight: '400' });
    window.execTerminal = this.terminal; // FIXME tragique, for acceptance tests…?

    // Sets the foreground colour to Structure’s ui-gray-400
    this.terminal.write('\x1b[38;2;142;150;163m');
    this.terminal.writeln('Select a task to start your session.');
  },

  actions: {
    setTaskState({ allocationSpecified, taskState }) {
      this.taskState = taskState;

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

      this.terminal.onKey(e => {
        if (this.socketOpen) {
          this.handleSocketKeyEvent(e);
        } else {
          this.handleCommandKeyEvent(e);
        }
      });

      this.terminal.simulateCommandKeyEvent = this.handleCommandKeyEvent.bind(this);
    },
  },

  openAndConnectSocket() {
    this.socket = this.sockets.getTaskStateSocket(this.taskState, this.command);

    this.socket.onmessage = e => {
      const json = JSON.parse(e.data);
      this.terminal.write(atob(json.stdout.data));
    };

    this.socket.onclose = e => {
      this.terminal.writeln('');
      this.terminal.write('\x1b[38;2;142;150;163m');
      this.terminal.writeln('The connection has closed.');
      // FIXME interpret different close events
    };
  },

  handleCommandKeyEvent(e) {
    if (e.domEvent.key === 'Enter') {
      this.openAndConnectSocket();
      this.terminal.writeln('');
      this.socketOpen = true;
    } else if (e.domEvent.key === 'Backspace') {
      if (this.command.length > 0) {
        this.terminal.write('\b \b');
        this.command = this.command.slice(0, -1);
      }
    } else if (e.key.length > 0) {
      this.terminal.write(e.key);
      this.command = `${this.command}${e.key}`;
    }
  },

  handleSocketKeyEvent(e) {
    this.socket.send(JSON.stringify({ stdin: { data: btoa(e.key) } }));
  },
});
