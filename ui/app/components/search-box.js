import { reads } from '@ember/object/computed';
import Component from '@ember/component';
import { run } from '@ember/runloop';

export default Component.extend({
  // Passed to the component (mutable)
  searchTerm: null,

  // Used as a debounce buffer
  _searchTerm: reads('searchTerm'),

  // Used to throttle sets to searchTerm
  debounce: 150,

  // A hook that's called when the search value changes
  onChange() {},

  classNames: ['search-box', 'field', 'has-addons'],

  actions: {
    setSearchTerm(e) {
      this.set('_searchTerm', e.target.value);
      run.debounce(this, updateSearch, this.debounce);
    },

    clear() {
      this.set('_searchTerm', '');
      run.debounce(this, updateSearch, this.debounce);
    },
  },
});

function updateSearch() {
  const newTerm = this._searchTerm;
  this.onChange(newTerm);
  this.set('searchTerm', newTerm);
}
