/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export const ListAccordionAccordionBody = <template>
  {{#if @isOpen}}
    <div
      data-test-accordion-body
      class="accordion-body {{if @fullBleed 'is-full-bleed'}}"
    >
      {{yield}}
    </div>
  {{/if}}
</template>;

export default ListAccordionAccordionBody;
