const ANSI_MOVE_CURSOR_UP_ONE = '\x1b[1A';

function ansi_move_cursor_right_n(n) {
  return `\x1b[${n}C`;
}

export default class ExecCommandEditorXtermAdapter {
  constructor(terminal, setCommandCallback, command) {
    this.terminal = terminal;
    this.setCommandCallback = setCommandCallback;

    this.command = command;

    this.keyListener = terminal.onKey(e => {
      this.handleKeyEvent(e);
    });

    terminal.simulateCommandKeyEvent = this.handleKeyEvent.bind(this);
  }

  handleKeyEvent(e) {
    if (e.domEvent.key === 'Enter') {
      this.terminal.writeln('');
      this.setCommandCallback(this.command);
      this.keyListener.dispose();
    } else if (e.domEvent.key === 'Backspace') {
      if (this.command.length > 0) {
        const cursorX = this.terminal.buffer.cursorX;

        if (cursorX === 0) {
          this.terminal.write(ANSI_MOVE_CURSOR_UP_ONE);
          this.terminal.write(ansi_move_cursor_right_n(this.terminal.cols));
          this.terminal.write(' ');
        } else {
          this.terminal.write('\b \b');
        }

        this.command = this.command.slice(0, -1);
      }
    } else if (e.key.length > 0) {
      this.terminal.write(e.key);
      this.command = `${this.command}${e.key}`;
    }
  }

  destroy() {
    this.keyListener.dispose();
  }
}
