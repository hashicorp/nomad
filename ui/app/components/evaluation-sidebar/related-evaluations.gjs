/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { fn } from '@ember/helper';
import { and } from 'ember-truth-helpers';
import { didUpdate } from '@ember/render-modifiers';
import onResize from 'ember-on-resize-modifier/modifiers/on-resize';
import EvaluationSidebarEvaluationActor from 'nomad-ui/components/evaluation-sidebar/evaluation-actor';
import ProvidersActorsRelationships from 'nomad-ui/components/providers/actors-relationships';

export const EvaluationSidebarRelatedEvaluations = <template>
  <div class="boxed-section">
    <div class="boxed-section-head">
      Related Evaluations
    </div>
    <div
      class="boxed-section-body related-evaluations"
      data-test-eval-container
    >
      <div class="sidebar-content" {{onResize @fns.handleResize}}>
        {{#if (and @data.width @data.height)}}
          <ProvidersActorsRelationships as |actors|>
            <svg
              width={{@data.width}}
              height="100%"
              style="z-index: 10; inset: 0; position: absolute; pointer-events: none"
              {{didUpdate actors.fns.recalcCurves @data.width}}
            >
              {{#each actors.data.relationships as |r|}}
                <path
                  d={{r.d}}
                  stroke="#7E8FA8"
                  strokeWidth="1"
                  fill="none"
                ></path>
                <circle
                  cx={{r.sx}}
                  cy={{r.sy}}
                  r="4"
                  fill="white"
                  stroke="black"
                ></circle>
                <circle
                  cx={{r.ex}}
                  cy={{r.ey}}
                  r="4"
                  fill="white"
                  stroke="black"
                ></circle>
              {{/each}}
            </svg>
          </ProvidersActorsRelationships>
        {{/if}}
        <div>
          <EvaluationSidebarEvaluationActor
            @eval={{@data.parentEvaluation}}
            @activeEvaluationID={{@data.activeEvaluationID}}
            @onClick={{fn @fns.handleEvaluationClick @data.parentEvaluation}}
          />
        </div>
        {{#each @data.descendentsMap as |evals|}}
          <div class="evaluation-actors">
            {{#each evals as |eval|}}
              <EvaluationSidebarEvaluationActor
                @eval={{eval.data}}
                @activeEvaluationID={{@data.activeEvaluationID}}
                @onClick={{fn @fns.handleEvaluationClick eval.data}}
              />
            {{/each}}
          </div>
        {{/each}}
      </div>
    </div>
  </div>
</template>;

export default EvaluationSidebarRelatedEvaluations;
