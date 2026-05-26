/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import MetadataKv from 'nomad-ui/components/metadata-kv';

const AttributesSection = <template>
  {{#each @attributes.files as |file|}}
    <MetadataKv
      @prefix={{file.prefix}}
      @key={{file.name}}
      @value={{file.variable.value}}
      @editable={{@editable}}
      @onKVSave={{@onKVSave}}
      @copyable={{@copyable}}
    />
  {{/each}}
  {{#each-in @attributes.children as |key value|}}
    <AttributesSection
      @attributes={{value}}
      @key={{key}}
      @value={{value}}
      @editable={{@editable}}
      @onKVSave={{@onKVSave}}
      @copyable={{@copyable}}
    />
  {{/each-in}}
</template>;

export default AttributesSection;
