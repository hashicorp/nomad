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

  classNames: ['search-box', 'field', 'has-addons'],

  actions: {
    setSearchTerm(e) {
      this.set('_searchTerm', e.target.value);
      run.debounce(this, updateSearch, this.get('debounce'));
    },
  },
});

function updateSearch() {
  this.set('searchTerm', this.get('_searchTerm'));
}
