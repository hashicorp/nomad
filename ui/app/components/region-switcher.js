/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { computed } from '@ember/object';
import { inject as service } from '@ember/service';
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

  async gotoRegion(region) {
    // Note: redundant but as long as we're using PowerSelect, the implicit set('activeRegion')
    // is not something we can await, so we do it explicitly here.
    this.system.set('activeRegion', region);
    await this.get('token.fetchSelfTokenAndPolicies').perform().catch();

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
