import Component from '@ember/component';
import { FitAddon } from 'xterm-addon-fit';
import $ from 'jquery';

export default Component.extend({
  classNames: ['terminal-container'],

  didInsertElement() {
    let fitAddon = new FitAddon();
    this.fitAddon = fitAddon;
    this.terminal.loadAddon(fitAddon);

    this.terminal.open(this.element.querySelector('.terminal'));

    fitAddon.fit();

    this._windowResizeHandler = this.windowResizeHandler.bind(this);
    $(window).on('resize', this._windowResizeHandler);
  },

  willDestroyElement() {
    $(window).off('resize', this._windowResizeHandler);
  },

  windowResizeHandler(e) {
    this.fitAddon.fit();
    if (this.terminal.resized) {
      this.terminal.resized(e);
    }
  },
});
