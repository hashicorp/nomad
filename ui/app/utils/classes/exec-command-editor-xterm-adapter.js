const REVERSE_WRAPAROUND_MODE = '\x1b[?45h';
const BACKSPACE_ONE_CHARACTER = '\x08 \x08';
const MOVE_CURSOR_BACKWARD = '\x1b[D';
const MOVE_CURSOR_FORWARD = '\x1b[C';
const INSERT_BLANK_CHARACTER = '\x1b[@';

export default class ExecCommandEditorXtermAdapter {
  insertionOffsetFromEnd = 0;

  constructor(terminal, setCommandCallback, command) {
    this.terminal = terminal;
    this.setCommandCallback = setCommandCallback;

    this.command = command;

    this.keyListener = terminal.onKey(e => {
      this.handleKeyEvent(e);
    });

    // Allows tests to bypass synthetic keyboard event restrictions
    terminal.simulateCommandKeyEvent = this.handleKeyEvent.bind(this);

    terminal.write(REVERSE_WRAPAROUND_MODE);
  }

  handleKeyEvent(e) {
    // Issue to handle arrow keys etc: https://github.com/hashicorp/nomad/issues/7463
    if (e.domEvent.key === 'u' && e.domEvent.ctrlKey) {
      for (let i = 0; i < this.command.length; i++) {
        this.terminal.write(BACKSPACE_ONE_CHARACTER);
      }

      this.command = '';
    } else if (e.domEvent.key === 'ArrowLeft') {
      if (this.insertionOffsetFromEnd < this.command.length) {
        this.terminal.write(MOVE_CURSOR_BACKWARD);
        this.insertionOffsetFromEnd += 1;
      }
    } else if (e.domEvent.key === 'ArrowRight') {
      if (this.insertionOffsetFromEnd > 0) {
        this.terminal.write(MOVE_CURSOR_FORWARD);
        this.insertionOffsetFromEnd -= 1;
      }
    } else if (e.domEvent.key === 'Enter') {
      this.terminal.writeln('');
      this.setCommandCallback(this.command);
      this.keyListener.dispose();
    } else if (e.domEvent.key === 'Backspace') {
      if (this.command.length > 0) {
        if (this.insertionOffsetFromEnd == 0) {
          this.terminal.write(BACKSPACE_ONE_CHARACTER);
          this.command = this.command.slice(0, -1);
        } else if (this.insertionOffsetFromEnd < this.command.length) {
          this.terminal.write(MOVE_CURSOR_BACKWARD);

          const commandLength = this.command.length;
          const insertionOffsetFromStart = commandLength - this.insertionOffsetFromEnd;
          const commandBeforeInsertionOffset = this.command.substring(
            0,
            insertionOffsetFromStart - 1
          );
          const commandAfterInsertionOffset = this.command.substring(insertionOffsetFromStart);

          this.command = `${commandBeforeInsertionOffset}${commandAfterInsertionOffset}`;

          this.terminal.write(commandAfterInsertionOffset);
          this.terminal.write(' ');

          for (let i = 0; i < commandAfterInsertionOffset.length + 1; i++) {
            this.terminal.write(MOVE_CURSOR_BACKWARD);
          }
        }
      }
    } else if (e.key.length > 0) {
      if (this.insertionOffsetFromEnd > 0) {
        this.terminal.write(INSERT_BLANK_CHARACTER);
      }

      this.terminal.write(e.key);

      const commandLength = this.command.length;
      const insertionOffsetFromStart = commandLength - this.insertionOffsetFromEnd;
      const commandBeforeInsertionOffset = this.command.substring(0, insertionOffsetFromStart);
      const commandAfterInsertionOffset = this.command.substring(insertionOffsetFromStart);

      this.command = `${commandBeforeInsertionOffset}${e.key}${commandAfterInsertionOffset}`;
    }
  }

  destroy() {
    this.keyListener.dispose();
  }
}
