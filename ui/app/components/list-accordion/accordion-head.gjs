/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { fn } from '@ember/helper';
import { on } from '@ember/modifier';
import { or } from 'ember-truth-helpers';

export const ListAccordionAccordionHead = <template>
  <div
    class="accordion-head
      {{if @isOpen '' 'is-light'}}
      {{if @isExpandable '' 'is-inactive'}}"
    data-test-accordion-head
  >
    <div class="accordion-head-content">
      {{yield}}
    </div>
    <button
      data-test-accordion-toggle
      data-test-accordion-summary-chart={{@buttonType}}
      class="button is-light is-compact pull-right accordion-toggle
        {{unless @isExpandable 'is-invisible'}}"
      {{on "click" (fn (if @isOpen @onClose @onOpen) @item)}}
      type="button"
    >
      {{or @buttonLabel "toggle"}}
    </button>
  </div>
</template>;

export default ListAccordionAccordionHead;
