/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import KEYS from 'nomad-ui/utils/keys';

const REVERSE_WRAPAROUND_MODE = '\x1b[?45h';
const BACKSPACE_ONE_CHARACTER = '\x08 \x08';

// eslint-disable-next-line no-control-regex
const UNPRINTABLE_CHARACTERS_REGEX = /[\x00-\x1F]/g;

export default class ExecCommandEditorXtermAdapter {
  constructor(terminal, setCommandCallback, command) {
    this.terminal = terminal;
    this.setCommandCallback = setCommandCallback;

    this.command = command;

    this.dataListener = terminal.onData((data) => {
      this.handleDataEvent(data);
    });

    // Allows tests to bypass synthetic keyboard event restrictions
    terminal.simulateCommandDataEvent = this.handleDataEvent.bind(this);

    terminal.write(REVERSE_WRAPAROUND_MODE);
  }

  handleDataEvent(data) {
    if (
      data === KEYS.LEFT_ARROW ||
      data === KEYS.UP_ARROW ||
      data === KEYS.RIGHT_ARROW ||
      data === KEYS.DOWN_ARROW
    ) {
      // Ignore arrow keys
    } else if (data === KEYS.CONTROL_U) {
      this.terminal.write(BACKSPACE_ONE_CHARACTER.repeat(this.command.length));
      this.command = '';
    } else if (data === KEYS.ENTER) {
      this.terminal.writeln('');
      this.setCommandCallback(this.command);
      this.dataListener.dispose();
    } else if (data === KEYS.DELETE) {
      if (this.command.length > 0) {
        this.terminal.write(BACKSPACE_ONE_CHARACTER);
        this.command = this.command.slice(0, -1);
      }
    } else if (data.length > 0) {
      const strippedData = data.replace(UNPRINTABLE_CHARACTERS_REGEX, '');
      this.terminal.write(strippedData);
      this.command = `${this.command}${strippedData}`;
    }
  }

  destroy() {
    this.dataListener.dispose();
  }
}
