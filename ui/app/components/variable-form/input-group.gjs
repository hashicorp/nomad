/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { HdsFormMaskedInputField } from '@hashicorp/design-system-components/components';

export const VariableFormInputGroup = <template>
  <label class="value-label" ...attributes>
    <HdsFormMaskedInputField
      @isMultiline={{true}}
      @value={{@entry.value}}
      @onInput={{@onInput}}
      @onChange={{@onInput}}
      rows="1"
      class="hds-typography-code-200"
      autocomplete="new-password"
      data-test-var-value
      as |F|
    >
      <F.Label>Value</F.Label>
    </HdsFormMaskedInputField>
  </label>
</template>;

export default VariableFormInputGroup;
