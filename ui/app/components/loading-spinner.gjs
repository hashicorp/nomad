/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { on } from '@ember/modifier';
import { HdsIcon } from '@hashicorp/design-system-components/components';

export default class LoadingSpinner extends Component {
  @tracked paused = false;

  togglePaused = () => {
    this.paused = !this.paused;
  };

  <template>
    <div
      class="loading-spinner {{if this.paused 'paused'}}"
      {{on "click" this.togglePaused}}
      ...attributes
    >
      <div class="cube-and-logo">
        <div class="cube">
          <div class="side side-1"></div>
          <div class="side side-2"></div>
          <div class="side side-3"></div>
          <div class="side side-4"></div>
        </div>
        <div class="logo-container">
          <HdsIcon @name="nomad" class="icon" />
        </div>
      </div>
    </div>
  </template>
}
