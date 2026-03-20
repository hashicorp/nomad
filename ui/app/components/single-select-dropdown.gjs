/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import PowerSelect from 'ember-power-select/components/power-select';

export default class SingleSelectDropdown extends Component {
  get activeOption() {
    return this.args.options?.find?.(
      (option) => option.key === this.args.selection,
    );
  }

  get ariaLabel() {
    return `label-single-select-dropdown-${this.args.label}`;
  }

  get searchEnabled() {
    return (this.args.options?.length ?? 0) > 10;
  }

  setSelection = ({ key }) => {
    this.args.onSelect?.(key);
  };

  <template>
    <div data-test-single-select-dropdown class="dropdown" ...attributes>
      <PowerSelect
        @ariaLabel={{this.ariaLabel}}
        @ariaLabelledBy={{this.ariaLabel}}
        @options={{@options}}
        @disabled={{@disabled}}
        @selected={{this.activeOption}}
        @searchEnabled={{this.searchEnabled}}
        @searchField="label"
        @onChange={{this.setSelection}}
        @dropdownClass="dropdown-options"
        as |option|
      >
        <span class="ember-power-select-prefix">{{@label}}: </span>
        <span class="dropdown-label" data-test-dropdown-option={{option.key}}>
          {{option.label}}
        </span>
      </PowerSelect>
    </div>
  </template>
}
