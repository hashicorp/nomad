/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class LoadingSpinner extends Component {
  @tracked paused = false;

  @action
  togglePaused() {
    this.paused = !this.paused;
  }
}
