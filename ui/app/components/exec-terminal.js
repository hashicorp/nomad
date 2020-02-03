import Component from '@ember/component';
import { FitAddon } from 'xterm-addon-fit';

export default Component.extend({
  classNames: ['terminal-container'],

  didInsertElement() {
    const fitAddon = new FitAddon();
    this.terminal.loadAddon(fitAddon);

    this.terminal.open(this.element.querySelector('.terminal'));

    fitAddon.fit();
  },
});
