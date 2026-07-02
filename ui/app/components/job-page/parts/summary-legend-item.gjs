/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { concat } from '@ember/helper';
import { HdsIcon } from '@hashicorp/design-system-components/components';

export const JobPagePartsSummaryLegendItem = <template>
  <div class="legend-item">
    <span
      class="color-swatch
        {{if @datum.className @datum.className (concat 'swatch-' @index)}}"
    ></span>
    <span class="text">
      <span class="value" data-test-legend-value={{@datum.className}}>
        {{@datum.value}}
      </span>
      <span>
        {{@datum.label}}
      </span>
    </span>
    {{#if @datum.help}}
      <span
        class="tooltip multiline"
        role="tooltip"
        aria-label="{{@datum.help}}"
      >
        <HdsIcon @name="info" @color="faint" />
      </span>
    {{/if}}
  </div>
</template>;

export default JobPagePartsSummaryLegendItem;
