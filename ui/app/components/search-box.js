/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { reads } from '@ember/object/computed';
import Component from '@ember/component';
import { action } from '@ember/object';
import { debounce } from '@ember/runloop';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('search-box', 'field', 'has-addons')
export default class SearchBox extends Component {
  // Passed to the component (mutable)
  searchTerm = null;

  // Used as a debounce buffer
  @reads('searchTerm') _searchTerm;

  // Used to throttle sets to searchTerm
  debounce = 150;

  // A hook that's called when the search value changes
  onChange() {}

  @action
  setSearchTerm(e) {
    this.set('_searchTerm', e.target.value);
    debounce(this, updateSearch, this.debounce);
  }

  @action
  clear() {
    this.set('_searchTerm', '');
    debounce(this, updateSearch, this.debounce);
  }
}

function updateSearch() {
  const newTerm = this._searchTerm;
  this.onChange(newTerm);
  this.set('searchTerm', newTerm);
}
