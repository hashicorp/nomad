/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { debounce } from '@ember/runloop';

const DEBOUNCE_MS = 500;

export default class JobSearchBoxComponent extends Component {
  @service keyboard;

  element = null;

  @action
  updateSearchText(event) {
    debounce(this, this.sendUpdate, event.target.value, DEBOUNCE_MS);
  }

  sendUpdate(value) {
    this.args.onSearchTextChange(value);
  }

  @action
  focus(element) {
    element.focus();
    // Because the element is an input,
    // and the "hide hints" part of our keynav implementation is on keyUp,
    // but the focus action happens on keyDown,
    // and the keynav explicitly ignores key input while focused in a text input,
    // we need to manually hide the hints here.
    this.keyboard.displayHints = false;
  }
}
