/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class GlobalSearchComponent extends Component {
  @tracked expanded = false;

  @action updateSearchText() {
    console.log('updateSearchText()');
  }

  @action focus(element) {
    console.log('.focus()', element);
    element.focus();
    this.expand();
  }

  @action
  expand() {
    console.log('.expand()');
    this.expanded = true;
  }
  @action
  retract() {
    console.log('.retract()');
    this.expanded = false;
  }
}
