/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import AttributesTable from 'nomad-ui/components/attributes-table';

export const JobPagePartsMeta = <template>
  {{#if @meta.structured}}
    <div class="boxed-section" ...attributes>
      <div class="boxed-section-head">
        Meta
      </div>
      <div class="boxed-section-body is-full-bleed">
        <AttributesTable
          data-test-meta
          @attributePairs={{@meta.structured.root}}
          @class="attributes-table"
        />
      </div>
    </div>
  {{/if}}
</template>;

export default JobPagePartsMeta;
