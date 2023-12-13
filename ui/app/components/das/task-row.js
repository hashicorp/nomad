/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class DasTaskRowComponent extends Component {
  @tracked height;

  get half() {
    return this.height / 2;
  }

  get borderCoverHeight() {
    return this.height - 2;
  }

  @action
  calculateHeight(element) {
    this.height = element.clientHeight + 1;
  }
}
