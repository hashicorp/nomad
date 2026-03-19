/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { or, not } from 'ember-truth-helpers';
import { HdsIcon } from '@hashicorp/design-system-components/components';

export const GlobalSearchTrigger = <template>
  <HdsIcon @name="search" @isInline={{true}} class="icon" />

  {{#unless @select.isOpen}}
    <span class="placeholder">Jump to</span>
  {{/unless}}

  {{#if (not (or @select.isActive @select.isOpen))}}
    <span class="shortcut" title="Type '/' to search">/</span>
  {{/if}}
</template>;

export default GlobalSearchTrigger;
