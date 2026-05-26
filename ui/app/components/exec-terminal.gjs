/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { FitAddon } from 'xterm-addon-fit';
import { didInsert } from '@ember/render-modifiers';
import windowResize from 'nomad-ui/modifiers/window-resize';

export default class ExecTerminal extends Component {
  fitAddon = null;
  beforeUnloadHandler = null;

  get terminal() {
    return this.args.terminal;
  }

  get socketOpen() {
    return this.args.socketOpen;
  }

  setupTerminal = (element) => {
    if (!this.terminal) {
      return;
    }

    const fitAddon = new FitAddon();
    this.fitAddon = fitAddon;
    this.terminal.loadAddon(fitAddon);
    this.terminal.open(element);
    fitAddon.fit();
    this.addExitHandler();
  };

  addExitHandler = () => {
    if (this.beforeUnloadHandler) {
      return;
    }

    this.beforeUnloadHandler = (event) => this.confirmExit(event);
    window.addEventListener('beforeunload', this.beforeUnloadHandler);
  };

  removeExitHandler = () => {
    if (!this.beforeUnloadHandler) {
      return;
    }

    window.removeEventListener('beforeunload', this.beforeUnloadHandler);
    this.beforeUnloadHandler = null;
  };

  confirmExit(event) {
    if (this.socketOpen) {
      event.preventDefault();
      return (event.returnValue = 'Are you sure you want to exit?');
    }
  }

  windowResizeHandler = (event) => {
    this.fitAddon?.fit();
    this.terminal?.resized?.(event);
  };

  willDestroy() {
    super.willDestroy(...arguments);
    this.removeExitHandler();
  }

  <template>
    <div class="terminal-container" ...attributes>
      <div
        class="terminal"
        {{didInsert this.setupTerminal}}
        {{windowResize this.windowResizeHandler}}
      ></div>
    </div>
  </template>
}
