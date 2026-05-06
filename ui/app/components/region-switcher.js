/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { action, computed } from '@ember/object';
import { service } from '@ember/service';
import classic from 'ember-classic-decorator';

@classic
export default class RegionSwitcher extends Component {
  @service system;
  @service router;
  @service store;
  @service token;

  @computed('system.regions')
  get sortedRegions() {
    return this.get('system.regions').toArray().sort();
  }

  @action
  async gotoRegion(region) {
    // Fetch token for the new region before transitioning
    await this.get('token.fetchSelfTokenAndPolicies').perform(region).catch();

    this.router.transitionTo({ queryParams: { region } });
  }

  get keyCommands() {
    if (this.sortedRegions.length <= 1) {
      return [];
    }
    return this.sortedRegions.map((region, iter) => {
      return {
        label: `Switch to ${region} region`,
        pattern: ['r', `${iter + 1}`],
        action: () => this.gotoRegion(region),
      };
    });
  }
}
