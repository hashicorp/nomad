/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class Alert extends Component {
  @tracked shouldShowAlert = true;

  @action
  dismissAlert() {
    this.shouldShowAlert = false;
  }
}
