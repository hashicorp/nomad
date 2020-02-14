import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
import escapeTaskName from 'nomad-ui/utils/escape-task-name';
import ExecCommandEditorXtermAdapter from 'nomad-ui/utils/classes/exec-command-editor-xterm-adapter';

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

    // this.terminal.write('\x1b[?45h;');
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

      new ExecCommandEditorXtermAdapter(
        this.terminal,
        this.openAndConnectSocket.bind(this),
        '/bin/bash'
      );

      // FIXME
      // this.terminal.simulateCommandKeyEvent = this.handleCommandKeyEvent.bind(this);
    },
  },

  openAndConnectSocket(command) {
    this.set('socketOpen', true);
    this.socket = this.sockets.getTaskStateSocket(this.taskState, command);

    this.terminal.onKey(e => {
      this.handleSocketKeyEvent(e);
    });

    this.socket.onmessage = e => {
      let json = JSON.parse(e.data);
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
