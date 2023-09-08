/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { set } from '@ember/object';
import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { action } from '@ember/object';
import Tether from 'tether';

export default class KeyboardShortcutsModalComponent extends Component {
  @service keyboard;
  @service config;

  blurHandler() {
    set(this, 'keyboard.displayHints', false);
  }

  constructor() {
    super(...arguments);
    set(this, '_blurHandler', this.blurHandler.bind(this));
    window.addEventListener('blur', this._blurHandler);
  }

  willDestroy() {
    super.willDestroy(...arguments);
    window.removeEventListener('blur', this._blurHandler);
  }

  escapeCommand = {
    label: 'Hide Keyboard Shortcuts',
    pattern: ['Escape'],
    action: () => {
      this.keyboard.shortcutsVisible = false;
    },
  };

  /**
   * commands: filter keyCommands to those that have an action and a label,
   * to distinguish between those that are just visual hints of existing commands
   */
  @computed('keyboard.keyCommands.[]')
  get commands() {
    return this.keyboard.keyCommands.reduce((memo, c) => {
      if (c.label && c.action && !memo.find((m) => m.label === c.label)) {
        memo.push(c);
      }
      return memo;
    }, []);
  }

  /**
   * hints: filter keyCommands to those that have an element property,
   * and then compute a position on screen to place the hint.
   */
  @computed('keyboard.{keyCommands.length,displayHints}')
  get hints() {
    if (this.keyboard.displayHints) {
      return this.keyboard.keyCommands.filter((c) => c.element);
    } else {
      return [];
    }
  }

  @action
  tetherToElement(element, hint, self) {
    if (!this.config.isTest) {
      let binder = new Tether({
        element: self,
        target: element,
        attachment: 'top left',
        targetAttachment: 'top left',
        targetModifier: 'visible',
      });
      hint.binder = binder;
    }
  }

  @action
  untetherFromElement(hint) {
    if (!this.config.isTest) {
      hint.binder.destroy();
    }
  }

  @action toggleListener() {
    this.keyboard.enabled = !this.keyboard.enabled;
  }
}
