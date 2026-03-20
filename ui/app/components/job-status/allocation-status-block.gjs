/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { htmlSafe } from '@ember/template';
import { eq, not } from 'ember-truth-helpers';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import ConditionalLinkTo from 'nomad-ui/components/conditional-link-to';
import JobStatusIndividualAllocation from 'nomad-ui/components/job-status/individual-allocation';

export default class JobStatusAllocationStatusBlock extends Component {
  get countToShow() {
    if (this.args.compact) {
      return 0;
    }

    const restWidth = 50;
    const restGap = 10;
    const countToShow = Math.floor(
      (this.args.width - (restWidth + restGap)) / 42,
    );

    return countToShow > 3 ? countToShow : 0;
  }

  get remaining() {
    return (this.args.count ?? 0) - this.countToShow;
  }

  get visibleAllocs() {
    return (this.args.allocs || []).slice(0, this.countToShow);
  }

  get firstAllocation() {
    return this.args.allocs?.[0];
  }

  get blockStyle() {
    return htmlSafe(`width: ${this.args.width}px`);
  }

  get representedAllocationClass() {
    return `represented-allocation rest ${this.args.status} ${this.args.health} ${this.args.canary}`;
  }

  get restQuery() {
    return {
      status: `["${this.args.status}"]`,
      version: `[${this.firstAllocation?.jobVersion}]`,
    };
  }

  <template>
    <div
      class="allocation-status-block {{unless this.countToShow 'rest-only'}}"
      style={{this.blockStyle}}
    >
      {{#if this.countToShow}}
        <div class="ungrouped-allocs">
          {{#each this.visibleAllocs as |allocation|}}
            <JobStatusIndividualAllocation
              @allocation={{allocation}}
              @status={{@status}}
              @health={{@health}}
              @canary={{@canary}}
              @steady={{@steady}}
            />
          {{/each}}
        </div>
      {{/if}}
      {{#if this.remaining}}

        <ConditionalLinkTo
          @condition={{not (eq @status "unplaced")}}
          @route="jobs.job.allocations"
          @model={{this.firstAllocation.job}}
          @query={{this.restQuery}}
          @class={{this.representedAllocationClass}}
          @label="View all {{@status}} allocations"
        >
          <span class="rest-count">{{#if
              this.countToShow
            }}+{{/if}}{{this.remaining}}</span>
          {{#unless @steady}}
            {{#if (eq @canary "canary")}}
              <span class="alloc-canary-indicator" />
            {{/if}}
            {{#if (eq @status "running")}}
              <span class="alloc-health-indicator">
                {{#if (eq @health "healthy")}}
                  <HdsIcon @name="check" @color="#25ba81" @isInline={{true}} />
                {{else if (eq @health "unhealthy")}}
                  <HdsIcon @name="x" @color="#c84034" @isInline={{true}} />
                {{else}}
                  <HdsIcon @name="running" @color="black" @isInline={{true}} />
                {{/if}}
              </span>
            {{/if}}
          {{/unless}}
        </ConditionalLinkTo>
      {{/if}}
    </div>
  </template>
}
