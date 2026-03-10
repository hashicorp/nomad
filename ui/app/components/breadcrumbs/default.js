/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { action } from '@ember/object';
import Component from '@glimmer/component';
import KeyboardShortcutModifier from 'nomad-ui/modifiers/keyboard-shortcut';
import { inject as service } from '@ember/service';

export default class BreadcrumbsTemplate extends Component {
  @service router;

  /**
   * The route name extracted from the crumb args array.
   * @returns {string}
   */
  get route() {
    return this.args.crumb?.args?.[0];
  }

  /**
   * The dynamic segments (models) for the route, extracted from the crumb args array.
   * @returns {Array}
   */
  get models() {
    return this.args.crumb?.args?.slice(1) ?? [];
  }

  @action
  traverseUpALevel() {
    this.router.transitionTo(this.route, ...this.models);
  }

  get maybeKeyboardShortcut() {
    return this.args.isOneCrumbUp() ? KeyboardShortcutModifier : null;
  }
}
