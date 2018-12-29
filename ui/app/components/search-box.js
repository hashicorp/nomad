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
      run.debounce(this, updateSearch, this.get('debounce'));
    },
  },
});

function updateSearch() {
  const newTerm = this.get('_searchTerm');
  this.onChange(newTerm);
  this.set('searchTerm', newTerm);
}
