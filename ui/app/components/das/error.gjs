/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { on } from '@ember/modifier';
import { HdsIcon } from '@hashicorp/design-system-components/components';

export default class DasError extends Component {
  dismissClicked = () => {
    this.args.proceed({ manuallyDismissed: true });
  };

  <template>
    <section class="das-error" data-test-recommendation-error>
      <section>
        <h3 data-test-headline>Recommendation error</h3>

        <p>
          There were errors processing applications:
        </p>

        <pre data-test-errors>{{@error}}</pre>
      </section>

      <HdsIcon @name="alert-circle-fill" />

      <section class="actions">
        <button
          data-test-dismiss
          class="button is-light"
          type="button"
          {{on "click" this.dismissClicked}}
        >Okay</button>
      </section>
    </section>
  </template>
}
