/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { or, not } from 'ember-truth-helpers';
import { HdsIcon } from '@hashicorp/design-system-components/components';

export default class GlobalSearchTrigger extends Component {
  get select() {
    return this.args.select;
  }

  <template>
    <HdsIcon @name="search" @isInline={{true}} class="icon" />

    {{#unless this.select.isOpen}}
      <span class="placeholder">Jump to</span>
    {{/unless}}

    {{#if (not (or this.select.isActive this.select.isOpen))}}
      <span class="shortcut" title="Type '/' to search">/</span>
    {{/if}}
  </template>
}
