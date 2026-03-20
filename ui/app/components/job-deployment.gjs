/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { on } from '@ember/modifier';
import { tracked } from '@glimmer/tracking';
import momentFromNow from 'ember-moment/helpers/moment-from-now';
import JobDeploymentDetails from 'nomad-ui/components/job-deployment-details';
import formatTs from 'nomad-ui/helpers/format-ts';

export default class JobDeployment extends Component {
  @tracked isOpen = false;

  toggleDetails = () => {
    this.isOpen = !this.isOpen;
  };

  <template>
    <div class="job-deployment boxed-section" ...attributes>
      <div class="boxed-section-head is-light inline-definitions">
        <span>{{@deployment.shortId}}</span>
        <span
          class="bumper-left tag {{@deployment.statusClass}}"
          data-test-deployment-status={{@deployment.statusClass}}
        >{{@deployment.status}}</span>
        {{#if @deployment.requiresPromotion}}
          <span
            data-test-promotion-required
            class="bumper-left badge is-warning is-hollow"
          >Requires Promotion</span>
        {{/if}}
        <span class="pull-right">
          <span class="pair is-faded">
            <span class="term">Version</span>
            <span
              class="has-emphasis"
              data-test-deployment-version
            >#{{@deployment.version.number}}</span>
            <span data-test-deployment-submit-time>|
              <span
                class="tooltip"
                aria-label={{formatTs @deployment.version.submitTime}}
              >
                {{momentFromNow @deployment.version.submitTime}}
              </span>
            </span>
          </span>
          <button
            data-test-deployment-toggle-details
            class="button is-light is-compact pull-right"
            {{on "click" this.toggleDetails}}
            type="button"
          >details</button>
        </span>
      </div>
      {{#if this.isOpen}}
        <div data-test-deployment-details class="boxed-section-body">
          <JobDeploymentDetails @deployment={{@deployment}} as |d|>
            <d.metrics />
            <d.taskGroups />
            <d.allocations />
          </JobDeploymentDetails>
        </div>
      {{/if}}
    </div>
  </template>
}
