/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { capitalize } from '@ember/string';
import { fn } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import { service } from '@ember/service';
import {
  HdsBadge,
  HdsIcon,
} from '@hashicorp/design-system-components/components';
import formatMonthTs from 'nomad-ui/helpers/format-month-ts';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import JobStatusAllocationStatusRow from 'nomad-ui/components/job-status/allocation-status-row';

export default class ChildJobRow extends Component {
  @service router;

  gotoJob = () => {
    const { job } = this.args;
    this.router.transitionTo('jobs.job.index', job.idWithNamespace);
  };

  get statusText() {
    return capitalize(this.args.job?.aggregateAllocStatus?.label || '');
  }

  <template>
    <tr data-test-job-row data-test-child-job-row ...attributes>
      <td
        data-test-job-name
        {{keyboardShortcut enumerated=true action=(fn this.gotoJob @job)}}
      >
        <LinkTo
          @route="jobs.job.index"
          @model={{@job.idWithNamespace}}
          class="is-primary"
        >
          {{@job.name}}

          {{#if @job.isPack}}
            <span data-test-pack-tag class="tag is-pack">
              <HdsIcon @name="box" @color="faint" />
              <span>Pack</span>
            </span>
          {{/if}}

        </LinkTo>
      </td>
      <td data-test-job-submit-time>
        {{formatMonthTs @job.submitTime}}
      </td>
      <td data-test-job-status>
        <span class={{@job.aggregateAllocStatus.label}}>
          <HdsBadge
            @text={{this.statusText}}
            @color={{@job.aggregateAllocStatus.state}}
            @size="large"
          />
        </span>
      </td>
      <td data-test-job-allocations>
        <div class="job-status-panel compact">
          <JobStatusAllocationStatusRow
            @allocBlocks={{@job.allocBlocks}}
            @steady={{true}}
            @compact={{true}}
            @completeAllocs={{@job.allocBlocks.complete.healthy.nonCanary.length}}
            @allocationTallyMode="complete"
            @groupCountSum={{@job.expectedRunningAllocCount}}
          />
        </div>
      </td>
    </tr>
  </template>
}
