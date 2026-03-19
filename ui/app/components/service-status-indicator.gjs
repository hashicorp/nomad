/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { eq } from 'ember-truth-helpers';
import formatTs from 'nomad-ui/helpers/format-ts';

export const ServiceStatusIndicator = <template>
  <span
    class="service-status-indicator status-{{@check.Status}}
      tooltip is-right-aligned"
    aria-label="{{@check.Status}} at {{formatTs @check.Timestamp}}"
  >
    {{#if (eq @check.Status "failure")}}
      &times;
    {{/if}}

    <span class="timestamp">
      <span>
        {{formatTs @check.Timestamp timeOnly=true}}
      </span>
    </span>
  </span>
</template>;

export default ServiceStatusIndicator;
