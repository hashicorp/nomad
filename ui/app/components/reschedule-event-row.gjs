/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { LinkTo } from '@ember/routing';
import formatTs from 'nomad-ui/helpers/format-ts';

const RescheduleEventRow = <template>
  <li class="timeline-note">
    {{#if @label}}
      <strong data-test-reschedule-label>{{@label}}</strong>
    {{/if}}
    {{formatTs @time}}
  </li>
  <li class="timeline-object" data-test-allocation={{@allocation.id}}>
    <div class="boxed-section">
      {{#unless @linkToAllocation}}
        <div class="boxed-section-head" data-test-row-heading>
          This Allocation
        </div>
      {{/unless}}
      <div class="boxed-section-body inline-definitions">
        <div class="columns">
          <div class="column is-centered is-minimum">
            <span
              data-test-allocation-status
              class="tag {{@allocation.statusClass}}"
            >
              {{@allocation.clientStatus}}
            </span>
          </div>
          <div class="column">
            <div class="boxed-section-row">
              <span class="pair">
                <span class="term">Allocation</span>
                {{#if @linkToAllocation}}
                  <LinkTo
                    @route="allocations.allocation"
                    @model={{@allocation.id}}
                  >
                    <code
                      data-test-allocation-link
                    >{{@allocation.shortId}}</code>
                  </LinkTo>
                {{else}}
                  <code data-test-allocation-link>{{@allocation.shortId}}</code>
                {{/if}}
              </span>
              <span class="pair">
                <span class="term">Client</span>
                <span>
                  <LinkTo
                    @route="clients.client"
                    @model={{@allocation.node.id}}
                    data-test-node-link
                  >
                    <code>{{@allocation.node.id}}</code>
                  </LinkTo>
                </span>
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </li>
</template>;

export default RescheduleEventRow;
