/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { action } from '@ember/object';
import Component from '@glimmer/component';
import KeyboardShortcutModifier from 'nomad-ui/modifiers/keyboard-shortcut';
import { inject as service } from '@ember/service';

export default class BreadcrumbsTemplate extends Component {
  @service router;

  @action
  traverseUpALevel(args) {
    const [path, ...rest] = args;
    this.router.transitionTo(path, ...rest);
  }

  get maybeKeyboardShortcut() {
    return this.args.isOneCrumbUp() ? KeyboardShortcutModifier : null;
  }
}
