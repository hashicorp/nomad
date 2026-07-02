/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { fn } from '@ember/helper';
import { on } from '@ember/modifier';
import { didInsert, willDestroy } from '@ember/render-modifiers';
import { eq } from 'ember-truth-helpers';
import ProvidersActorsRelationships from 'nomad-ui/components/providers/actors-relationships';
import StatusCell from 'nomad-ui/components/status-cell';

export const EvaluationSidebarEvaluationActor = <template>
  <ProvidersActorsRelationships as |actors|>
    <div
      class="related-evaluation
        {{if (eq @eval.id @activeEvaluationID) 'is-active'}}"
      data-eval={{@eval.id}}
      ...attributes
      {{didInsert (fn actors.fns.registerActor @eval)}}
      {{willDestroy (fn actors.fns.deregisterActor @eval)}}
    >
      {{#if (eq @eval.id @activeEvaluationID)}}
        <span data-test-rel-eval={{@eval.id}}>
          {{@eval.shortId}}
        </span>
      {{else}}
        <a data-test-rel-eval={{@eval.id}} {{on "click" @onClick}}>
          {{@eval.shortId}}
        </a>
      {{/if}}
      <span>
        <StatusCell @status={{@eval.status}} />
      </span>
    </div>
  </ProvidersActorsRelationships>
</template>;

export default EvaluationSidebarEvaluationActor;
