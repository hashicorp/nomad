/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { or } from 'ember-truth-helpers';
import PowerSelect from 'ember-power-select/components/power-select';
import keyboardCommands from 'nomad-ui/helpers/keyboard-commands';

export default class RegionSwitcher extends Component {
  @service system;
  @service router;
  @service store;
  @service token;

  get sortedRegions() {
    return [...this.system.regions].sort();
  }

  gotoRegion = async (region) => {
    // Note: redundant but as long as we're using PowerSelect, the implicit set('activeRegion')
    // is not something we can await, so we do it explicitly here.
    this.system.set('activeRegion', region);
    await this.token.fetchSelfTokenAndPolicies.perform().catch(() => {});

    this.router.transitionTo({ queryParams: { region } });
  };

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

  <template>
    {{keyboardCommands this.keyCommands}}

    {{#if this.system.shouldShowRegions}}
      <span data-test-region-switcher-parent ...attributes>
        <PowerSelect
          data-test-region-switcher
          @ariaLabel="label-region-switcher"
          @ariaLabelledBy="label-region-switcher"
          @tagName="div"
          @triggerClass={{@decoration}}
          @options={{this.sortedRegions}}
          @selected={{or this.system.activeRegion "Select a Region"}}
          @searchEnabled={{false}}
          @onChange={{this.gotoRegion}}
          as |region|
        >
          {{#if this.system.activeRegion}}
            <span class="ember-power-select-prefix">Region: </span>
          {{/if}}
          {{region}}
        </PowerSelect>
      </span>
    {{else if this.system.hasNonDefaultRegion}}
      <div class="navbar-item single-region" ...attributes>
        <span>Region: </span>{{this.system.activeRegion}}
      </div>
    {{/if}}
  </template>
}
