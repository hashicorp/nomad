/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Component from '@ember/component';
import { FitAddon } from 'xterm-addon-fit';
import WindowResizable from '../mixins/window-resizable';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

@classic
@classNames('terminal-container')
export default class ExecTerminal extends Component.extend(WindowResizable) {
  @service router;

  didInsertElement() {
    super.didInsertElement(...arguments);
    let fitAddon = new FitAddon();
    this.fitAddon = fitAddon;
    this.terminal.loadAddon(fitAddon);

    this.terminal.open(this.element.querySelector('.terminal'));

    fitAddon.fit();
    this.addExitHandler();
  }

  socketOpen = false;
  hasRemovedExitHandler = false;

  @action
  addExitHandler() {
    window.addEventListener('beforeunload', this.confirmExit.bind(this));
  }
  removeExitHandler() {
    if (!this.hasRemovedExitHandler) {
      window.removeEventListener('beforeunload', this.confirmExit.bind(this));
      this.hasRemovedExitHandler = true;
    }
  }

  /**
   *
   * @param {BeforeUnloadEvent} event
   * @returns {string}
   */
  confirmExit(event) {
    if (this.socketOpen) {
      event.preventDefault();
      return (event.returnValue = 'Are you sure you want to exit?');
    }
  }

  willDestroy() {
    super.willDestroy(...arguments);
    this.removeExitHandler();
  }

  windowResizeHandler(e) {
    this.fitAddon.fit();
    if (this.terminal.resized) {
      this.terminal.resized(e);
    }
  }
}
