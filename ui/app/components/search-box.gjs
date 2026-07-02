/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { debounce } from '@ember/runloop';
import { on } from '@ember/modifier';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import { didUpdate } from '@ember/render-modifiers';

export default class SearchBox extends Component {
  @tracked _searchTerm = null;

  debounce = 150;

  constructor() {
    super(...arguments);
    this.syncSearchTerm();
  }

  syncSearchTerm = () => {
    this._searchTerm = this.args.searchTerm;
  };

  setSearchTerm = (e) => {
    this._searchTerm = e.target.value;
    debounce(this, this.updateSearch, this.debounce);
  };

  clear = () => {
    this._searchTerm = '';
    debounce(this, this.updateSearch, this.debounce);
  };

  updateSearch = () => {
    const newTerm = this._searchTerm;
    this.args.onChange?.(newTerm);
  };

  <template>
    <div
      class="search-box field has-addons"
      {{didUpdate this.syncSearchTerm @searchTerm}}
      ...attributes
    >
      <div class="control">
        <span class="prefix-icon"><HdsIcon @name="search" /></span>
        <input
          class="input {{@inputClass}}"
          type="text"
          placeholder={{@placeholder}}
          aria-label={{@placeholder}}
          value={{this._searchTerm}}
          {{on "input" this.setSearchTerm}}
          size="1"
        />
        <button
          class="suffix-icon"
          {{on "click" this.clear}}
          type="button"
          title="Clear search"
        ><HdsIcon @name="x" /></button>
      </div>
    </div>
  </template>
}
