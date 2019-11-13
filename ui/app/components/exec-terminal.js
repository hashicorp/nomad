import Component from '@ember/component';
import { Terminal } from 'xterm';

export default Component.extend({
  didInsertElement() {
    this.terminal = new Terminal();
    this.terminal.open(this.element.querySelector('.terminal'));

    this.terminal.onKey(e => {
      this.socket.send(JSON.stringify({ stdin: { data: btoa(e.key) } }));
    });

    // FIXME this is a hack to provide access in an integration test 🧐
    window.xterm = this.terminal;

    this.socket.onmessage = e => {
      const json = JSON.parse(e.data);
      this.terminal.write(atob(json.stdout.data));
    };
  },
});
