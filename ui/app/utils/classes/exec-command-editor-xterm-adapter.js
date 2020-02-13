export default class ExecCommandEditorXtermAdapter {
  constructor(terminal, setCommandCallback) {
    this.terminal = terminal;
    this.setCommandCallback = setCommandCallback;

    this.command = '/bin/bash';

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
        this.terminal.write('\b \b');
        this.command = this.command.slice(0, -1);
      }
    } else if (e.key.length > 0) {
      this.terminal.write(e.key);
      this.command = `${this.command}${e.key}`;
    }
  }
}
