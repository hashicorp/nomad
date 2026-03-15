/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Modifier from 'ember-modifier';
import { registerDestructor } from '@ember/destroyable';

export default class KeyboardShortcutModifier extends Modifier {
  @service keyboard;
  @service router;
  _commands = [];
  _destructorRegistered = false;

  modify(
    element,
    _positional,
    {
      label,
      pattern = '',
      action = () => {},
      menuLevel = false,
      enumerated = false,
      exclusive = false,
    },
  ) {
    if (this._commands.length) {
      this.keyboard.removeCommands(this._commands);
    }

    this._commands = [
      {
        label,
        action,
        pattern,
        element,
        menuLevel,
        enumerated,
        exclusive,
      },
    ];

    this.keyboard.addCommands(this._commands);

    if (!this._destructorRegistered) {
      registerDestructor(this, () => {
        this.keyboard.removeCommands(this._commands);
      });
      this._destructorRegistered = true;
    }
  }
}
