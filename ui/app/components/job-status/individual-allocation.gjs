/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { get } from '@ember/object';
import { eq, not } from 'ember-truth-helpers';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import ConditionalLinkTo from 'nomad-ui/components/conditional-link-to';

export default class JobStatusIndividualAllocation extends Component {
  get allocationId() {
    return get(this.args.allocation, 'id');
  }

  get jobType() {
    return get(this.args.allocation, 'job.type');
  }

  get nodeName() {
    return get(this.args.allocation, 'node.name');
  }

  get groupName() {
    return get(this.args.allocation, 'taskGroup.name');
  }

  get taskGroups() {
    return get(this.args.allocation, 'job.taskGroups');
  }

  get shortId() {
    return get(this.args.allocation, 'shortId');
  }

  get showClient() {
    return this.jobType === 'system' || this.jobType === 'sysbatch';
  }

  get tooltipText() {
    if (this.showClient) {
      return `${this.nodeName} - ${this.shortId}`;
    } else if (this.groupName && this.taskGroups?.length > 1) {
      return `${this.groupName} - ${this.shortId}`;
    }

    return this.shortId;
  }

  get tooltip() {
    if (!this.tooltipText) {
      return null;
    }

    return {
      text: this.tooltipText,
      extraTippyOptions: {
        trigger: this.args.status === 'unplaced' ? 'manual' : undefined,
      },
    };
  }

  <template>
    <ConditionalLinkTo
      @condition={{not (eq @status "unplaced")}}
      @route="allocations.allocation"
      @model={{this.allocationId}}
      @class="represented-allocation {{@status}} {{@health}} {{@canary}}"
      @label="View allocation"
      @tooltip={{this.tooltip}}
    >
      {{#unless @steady}}
        {{#if (eq @canary "canary")}}
          <span class="alloc-canary-indicator" />
        {{/if}}
        {{#if (eq @status "running")}}
          <span class="alloc-health-indicator">
            {{#if (eq @health "healthy")}}
              <HdsIcon @name="check" @color="white" @isInline={{true}} />
            {{else if (eq @health "unhealthy")}}
              <HdsIcon @name="x" @color="white" @isInline={{true}} />
            {{else}}
              <HdsIcon @name="running" @color="white" @isInline={{true}} />
            {{/if}}
          </span>
        {{/if}}
      {{/unless}}
    </ConditionalLinkTo>
  </template>
}
