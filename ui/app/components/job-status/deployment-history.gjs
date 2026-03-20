/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { service } from '@ember/service';
import { get } from '@ember/object';
import { scheduleOnce } from '@ember/runloop';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { eq, or } from 'ember-truth-helpers';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import SearchBox from 'nomad-ui/components/search-box';
import formatTs from 'nomad-ui/helpers/format-ts';

const MAX_NUMBER_OF_EVENTS = 500;

export default class JobStatusDeploymentHistory extends Component {
  @service notifications;

  @tracked isHidden = false;
  @tracked errorState = null;
  @tracked searchTerm = '';

  constructor() {
    super(...arguments);

    this.isHidden = this.args.isHidden ?? false;
  }

  get job() {
    return get(this.args.deployment, 'job');
  }

  get deploymentVersion() {
    return get(this.args.deployment, 'versionNumber');
  }

  get jobAllocations() {
    return this.job?.get?.('allocations') || [];
  }

  get deploymentAllocations() {
    return (
      this.args.allocations ||
      this.jobAllocations.filter(
        (alloc) => alloc.jobVersion === this.deploymentVersion,
      )
    );
  }

  get history() {
    try {
      return this.deploymentAllocations
        .map((allocation) => {
          const states =
            allocation?.get?.('states') || allocation?.states || [];
          const stateList = states?.toArray?.() || states || [];

          return stateList
            .map((state) => state?.events?.toArray?.() || state?.events || [])
            .flat();
        })
        .flat()
        .filter(Boolean)
        .filter((taskEvent) => this.containsSearchTerm(taskEvent))
        .sort((left, right) => {
          const leftTime = left?.time?.valueOf?.() || left?.get?.('time') || 0;
          const rightTime =
            right?.time?.valueOf?.() || right?.get?.('time') || 0;
          return leftTime - rightTime;
        })
        .reverse()
        .slice(0, MAX_NUMBER_OF_EVENTS);
    } catch (error) {
      this.triggerError(error);
      return [];
    }
  }

  triggerError(error) {
    // eslint-disable-next-line ember/no-incorrect-calls-with-inline-anonymous-functions
    scheduleOnce('actions', this, () => {
      if (this.errorState === error) {
        return;
      }

      this.errorState = error;
      this.notifications.add({
        title: 'Could not fetch deployment history',
        message: error?.message || String(error),
        color: 'critical',
      });
    });
  }

  containsSearchTerm(taskEvent) {
    if (!taskEvent) {
      return false;
    }

    const lowerSearchTerm = this.searchTerm.toLowerCase();
    const message = (taskEvent.message || '').toLowerCase();
    const type = (taskEvent.type || '').toLowerCase();
    const allocationShortId =
      taskEvent.state?.allocation?.shortId?.toLowerCase?.() || '';

    return (
      message.includes(lowerSearchTerm) ||
      type.includes(lowerSearchTerm) ||
      allocationShortId.includes(lowerSearchTerm)
    );
  }

  toggleHidden = () => {
    this.isHidden = !this.isHidden;
  };

  setSearchTerm = (searchTerm) => {
    this.searchTerm = searchTerm;
  };

  <template>
    <div class="deployment-history {{if this.isHidden 'hidden'}}" ...attributes>
      <header>
        <h4 class="title is-5">
          <button class="button" {{on "click" this.toggleHidden}} type="button">
            {{or @title "Deployment History"}}
            {{#if this.isHidden}}
              <HdsIcon @name="chevron-down" />
            {{else}}
              <HdsIcon @name="chevron-up" />
            {{/if}}
          </button>
        </h4>
        {{#unless this.isHidden}}
          <SearchBox
            data-test-history-search
            @searchTerm={{this.searchTerm}}
            @onChange={{this.setSearchTerm}}
            @placeholder="Search events..."
          />
        {{/unless}}
      </header>
      {{#unless this.isHidden}}
        <div class="timeline-container">
          <ol class="timeline">
            {{#each this.history as |deploymentLog|}}
              <li
                class="timeline-object
                  {{if (eq deploymentLog.exitCode 1) 'error'}}"
              >
                <div class="boxed-section-head is-light">
                  <LinkTo
                    @route="allocations.allocation"
                    @model={{deploymentLog.state.allocation.id}}
                    class="allocation-reference"
                  >{{deploymentLog.state.allocation.shortId}}</LinkTo>
                  <span><strong>{{deploymentLog.type}}:</strong>
                    {{deploymentLog.message}}</span>
                  <span class="pull-right">
                    {{formatTs deploymentLog.time}}
                  </span>
                </div>
              </li>
            {{else}}
              {{#if this.errorState}}
                <li class="timeline-object">
                  <div class="boxed-section-head is-light">
                    <span>Error loading deployment history</span>
                  </div>
                </li>
              {{else if this.deploymentAllocations.length}}
                {{#if this.searchTerm}}
                  <li class="timeline-object" data-test-history-search-no-match>
                    <div class="boxed-section-head is-light">
                      <span>No events match {{this.searchTerm}}</span>
                    </div>
                  </li>
                {{else}}
                  <li class="timeline-object">
                    <div class="boxed-section-head is-light">
                      <span>No deployment events yet</span>
                    </div>
                  </li>
                {{/if}}
              {{else}}
                <li class="timeline-object">
                  <div class="boxed-section-head is-light">
                    <span>Loading deployment events</span>
                  </div>
                </li>
              {{/if}}
            {{/each}}
          </ol>
        </div>
      {{/unless}}
    </div>
  </template>
}
