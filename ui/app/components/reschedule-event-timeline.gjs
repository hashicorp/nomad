/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { and, not } from 'ember-truth-helpers';
import { service } from '@ember/service';
import { reverse } from '@nullvoxpopuli/ember-composable-helpers';
import momentFromNow from 'ember-moment/helpers/moment-from-now';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import formatTs from 'nomad-ui/helpers/format-ts';
import RescheduleEventRow from 'nomad-ui/components/reschedule-event-row';

export default class RescheduleEventTimeline extends Component {
  @service store;

  allocationCache = new Map();

  allocationForEvent = (event) => {
    const id = event?.previousAllocationId;
    if (!id) {
      return null;
    }

    if (this.allocationCache.has(id)) {
      return this.allocationCache.get(id);
    }

    const allocation =
      this.store.peekRecord('allocation', id) ||
      this.store.findRecord('allocation', id);
    this.allocationCache.set(id, allocation);
    return allocation;
  };

  <template>
    <ol class="timeline" ...attributes>
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
          @allocation={{this.allocationForEvent event}}
          @time={{event.time}}
        />
      {{/each}}
    </ol>
  </template>
}
