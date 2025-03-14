/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { inject as service } from '@ember/service';

export default class GlobalSearchComponent extends Component {
  @service history;
  @service keyboard;

  @tracked expanded = false;
  @tracked element;

  /**
   *
   * @param {InputEvent} event
   */
  @action updateSearchText(event) {
    console.log('updateSearchText()', event, event.key);
  }

  @action handleKeyUp(event, b, c) {
    console.log('hKU', event, b, c);
    if (event.key === 'Escape') {
      this.expanded = false; // TODO: can't use retract() because of the activeElement condition
    }
  }

  @action focus(element) {
    console.log('.focus()', element);
    element.focus();
    this.expand();
  }

  /**
   *
   * @param {FocusEvent} event
   */
  @action
  expand(event) {
    console.log('.expand()');
    this.element = event?.target;
    this.expanded = true;
    this.keyboard.displayHints = false;
  }
  @action
  retract() {
    console.log('.retract()'); //, this.element, document.activeElement);
    // TODO: bad way to do this!
    if (this.element && document.activeElement !== this.element) {
      this.expanded = false;
    }
  }
}
