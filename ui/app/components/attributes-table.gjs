/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import AttributesSection from 'nomad-ui/components/attributes-section';

export const AttributesTable = <template>
  <table
    class="table is-striped is-fixed is-compact is-darkened no-mobile-condense"
    ...attributes
  >
    <thead>
      <tr>
        <th class="is-one-third">Name</th>
        <th>Value</th>
      </tr>
    </thead>
    <tbody>
      <AttributesSection
        @attributes={{@attributePairs}}
        @editable={{@editable}}
        @onKVEdit={{@onKVEdit}}
        @onKVSave={{@onKVSave}}
        @copyable={{@copyable}}
      />
    </tbody>
  </table>
</template>;

export default AttributesTable;
