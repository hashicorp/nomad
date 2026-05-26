/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { on } from '@ember/modifier';
import { didInsert } from '@ember/render-modifiers';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

export default class DasDismissed extends Component {
  @localStorageProperty('nomadRecommendationDismssalUnderstood', false)
  explanationUnderstood;

  @tracked dismissInTheFuture = false;

  proceedAutomatically = () => {
    this.args.proceed({ manuallyDismissed: false });
  };

  understoodClicked = () => {
    this.explanationUnderstood = this.dismissInTheFuture;
    this.args.proceed({ manuallyDismissed: true });
  };

  toggleDismissInTheFuture = (event) => {
    this.dismissInTheFuture = event.target.checked;
  };

  <template>
    <section
      class="das-dismissed {{if this.explanationUnderstood 'understood'}}"
    >
      {{#if this.explanationUnderstood}}
        <h3 {{didInsert this.proceedAutomatically}}>Recommendation dismissed</h3>
      {{else}}
        <section>
          <h3>Recommendation dismissed</h3>

          <p>Nomad will not apply these resource change recommendations.</p>

          <p>To never get recommendations for this task group again, disable
            dynamic application sizing in the job definition.</p>
        </section>

        <section class="actions">
          <button
            data-test-understood
            class="button is-info"
            type="button"
            {{on "click" this.understoodClicked}}
          >Understood</button>
          <label>
            <input
              type="checkbox"
              checked={{this.dismissInTheFuture}}
              {{on "change" this.toggleDismissInTheFuture}}
            />
            Don't show this again
          </label>
        </section>
      {{/if}}
    </section>
  </template>
}
