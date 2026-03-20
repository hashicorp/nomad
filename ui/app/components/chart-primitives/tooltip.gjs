/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inc } from '@nullvoxpopuli/ember-composable-helpers';

export const ChartPrimitivesTooltip = <template>
  <div
    data-test-chart-tooltip
    class="chart-tooltip {{if @active 'active' 'inactive'}}"
    style={{@style}}
    ...attributes
  >
    <ol>
      {{#each @data as |props|}}
        {{yield props.series props.datum (inc props.index)}}
      {{/each}}
    </ol>
  </div>
</template>;

export default ChartPrimitivesTooltip;
