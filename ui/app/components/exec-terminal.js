/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { FitAddon } from 'xterm-addon-fit';
import WindowResizable from '../mixins/window-resizable';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('terminal-container')
export default class ExecTerminal extends Component.extend(WindowResizable) {
  didInsertElement() {
    super.didInsertElement(...arguments);
    let fitAddon = new FitAddon();
    this.fitAddon = fitAddon;
    this.terminal.loadAddon(fitAddon);

    this.terminal.open(this.element.querySelector('.terminal'));

    fitAddon.fit();
  }

  windowResizeHandler(e) {
    this.fitAddon.fit();
    if (this.terminal.resized) {
      this.terminal.resized(e);
    }
  }
}
