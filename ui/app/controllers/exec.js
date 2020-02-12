import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
import escapeTaskName from 'nomad-ui/utils/escape-task-name';

import { Terminal } from 'xterm';
import base64js from 'base64-js';
import { TextDecoderLite, TextEncoderLite } from 'text-encoder-lite';

const ANSI_UI_GRAY_400 = '\x1b[38;2;142;150;163m';
const ANSI_WHITE = '\x1b[0m';

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

    this.terminal.write(ANSI_UI_GRAY_400);
    this.terminal.writeln('Select a task to start your session.');
  },

  actions: {
    setTaskState({ allocationSpecified, taskState }) {
      this.set('taskState', taskState);

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
        `$ nomad alloc exec -i -t -task ${escapeTaskName(taskState.name)} ${
          taskState.allocation.shortId
        } `
      );

      this.terminal.write(ANSI_WHITE);

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
      this.terminal.write(decodeString(json.stdout.data));
    };

    this.socket.onclose = e => {
      this.terminal.writeln('');
      this.terminal.write(ANSI_UI_GRAY_400);
      this.terminal.writeln('The connection has closed.');
      // eslint-disable-next-line
      console.log('Socket close event', e);
      // FIXME interpret different close events
    };
  },

  handleCommandKeyEvent(e) {
    if (e.domEvent.key === 'Enter') {
      this.openAndConnectSocket();
      this.terminal.writeln('');
      this.set('socketOpen', true);
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
    this.socket.send(JSON.stringify({ stdin: { data: encodeString(e.key) } }));
    // FIXME this is untested, difficult with restriction on simulating key events
  },
});

function encodeString(string) {
  var encoded = new TextEncoderLite('utf-8').encode(string);
  return base64js.fromByteArray(encoded);
}

function decodeString(b64String) {
  var uint8array = base64js.toByteArray(b64String);
  return new TextDecoderLite('utf-8').decode(uint8array);
}
