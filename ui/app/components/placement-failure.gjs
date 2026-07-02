/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { HdsBadge } from '@hashicorp/design-system-components/components';

const isZero = (value) => value === 0;
const plusOne = (value) => value + 1;
const pluralizedNode = (count) => (count === 1 ? 'node' : 'nodes');

export default class PlacementFailure extends Component {
  get placementFailures() {
    return this.args.taskGroup?.placementFailures ?? this.args.failedTGAlloc;
  }

  <template>
    {{#if this.placementFailures}}
      {{#let this.placementFailures as |failures|}}
        <h3 class="title is-5" data-test-placement-failure-task-group>
          {{failures.name}}
          <HdsBadge
            data-test-placement-failure-coalesced-failures
            @color="critical"
            @type="outlined"
            @size="small"
            @text="{{plusOne failures.coalescedFailures}} unplaced"
          />
        </h3>

        <ul class="simple-list">
          {{#if (isZero failures.nodesEvaluated)}}
            <li data-test-placement-failure-no-evaluated-nodes>
              No nodes were eligible for evaluation
            </li>
          {{/if}}

          {{#each-in failures.nodesAvailable as |datacenter available|}}
            {{#if (isZero available)}}
              <li
                data-test-placement-failure-no-nodes-available="{{datacenter}}"
              >
                No nodes are available in datacenter
                {{datacenter}}
              </li>
            {{/if}}
          {{/each-in}}

          {{#each-in failures.classFiltered as |class count|}}
            <li data-test-placement-failure-class-filtered="{{class}}">
              Class
              {{class}}
              filtered
              {{count}}
              {{pluralizedNode count}}
            </li>
          {{/each-in}}

          {{#each-in failures.constraintFiltered as |constraint count|}}
            <li
              data-test-placement-failure-constraint-filtered="{{constraint}}"
            >
              Constraint
              <code>{{constraint}}</code>
              filtered
              {{count}}
              {{pluralizedNode count}}
            </li>
          {{/each-in}}

          {{#if failures.nodesExhausted}}
            <li data-test-placement-failure-nodes-exhausted>
              Resources exhausted on
              {{failures.nodesExhausted}}
              {{pluralizedNode failures.nodesExhausted}}
            </li>
          {{/if}}

          {{#each-in failures.classExhausted as |class count|}}
            <li data-test-placement-failure-class-exhausted="{{class}}">
              Class
              {{class}}
              exhausted on
              {{count}}
              {{pluralizedNode count}}
            </li>
          {{/each-in}}

          {{#each-in failures.dimensionExhausted as |dimension count|}}
            <li data-test-placement-failure-dimension-exhausted="{{dimension}}">
              Dimension
              {{dimension}}
              exhausted on
              {{count}}
              {{pluralizedNode count}}
            </li>
          {{/each-in}}

          {{#each-in failures.quotaExhausted as |quota dimension|}}
            <li data-test-placement-failure-quota-exhausted="{{quota}}">
              Quota limit hit
              {{dimension}}
            </li>
          {{/each-in}}

          {{#each-in failures.scores as |name score|}}
            <li data-test-placement-failure-scores="{{name}}">
              Score
              {{name}}
              =
              {{score}}
            </li>
          {{/each-in}}
        </ul>
      {{/let}}
    {{/if}}
  </template>
}
