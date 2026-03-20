/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { on } from '@ember/modifier';

const JobPagePartsError = <template>
  {{#if @errorMessage}}
    <div class="notification is-danger">
      <div class="columns">
        <div class="column">
          <h3
            data-test-job-error-title
            class="title is-4"
          >{{@errorMessage.title}}</h3>
          <p data-test-job-error-body>{{@errorMessage.description}}</p>
        </div>
        <div class="column is-centered is-minimum">
          <button
            data-test-job-error-close
            class="button is-danger"
            {{on "click" @onDismiss}}
            type="button"
          >Okay</button>
        </div>
      </div>
    </div>
  {{/if}}
</template>;

export default JobPagePartsError;
