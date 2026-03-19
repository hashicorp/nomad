/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { and, not } from 'ember-truth-helpers';
import reverse from 'ember-composable-helpers/helpers/reverse';
import momentFromNow from 'ember-moment/helpers/moment-from-now';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import formatTs from 'nomad-ui/helpers/format-ts';
import RescheduleEventRow from 'nomad-ui/components/reschedule-event-row';

export const RescheduleEventTimeline = <template>
  <ol class="timeline">
    {{#if @allocation.nextAllocation}}
      <RescheduleEventRow
        @label="Next Allocation"
        @allocation={{@allocation.nextAllocation}}
        @time={{@allocation.nextAllocation.modifyTime}}
      />
    {{/if}}
    {{#if @allocation.hasStoppedRescheduling}}
      <li class="timeline-note" data-test-stop-warning>
        <HdsIcon
          @name="alert-triangle-fill"
          @color="warning"
          @isInline={{true}}
          class="icon-vertical-bump-down"
        />
        Nomad has stopped attempting to reschedule this allocation.
      </li>
    {{/if}}
    {{#if
      (and
        @allocation.followUpEvaluation.waitUntil
        (not @allocation.nextAllocation)
      )
    }}
      <li class="timeline-note" data-test-attempt-notice>
        <HdsIcon @name="clock" @color="info" @isInline={{true}} />
        Nomad will attempt to reschedule
        <span
          class="tooltip"
          aria-label="{{formatTs @allocation.followUpEvaluation.waitUntil}}"
        >
          {{momentFromNow
            @allocation.followUpEvaluation.waitUntil
            interval=1000
          }}
        </span>
      </li>
    {{/if}}
    <RescheduleEventRow
      @allocation={{@allocation}}
      @linkToAllocation={{false}}
      @time={{@allocation.modifyTime}}
    />

    {{#each (reverse @allocation.rescheduleEvents) as |event|}}
      <RescheduleEventRow
        @allocationId={{event.previousAllocationId}}
        @time={{event.time}}
      />
    {{/each}}
  </ol>
</template>;

export default RescheduleEventTimeline;
