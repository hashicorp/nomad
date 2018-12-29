import Ember from 'ember';

const { Component, computed, run } = Ember;

export default Component.extend({
  // Passed to the component (mutable)
  searchTerm: null,

  // Used as a debounce buffer
  _searchTerm: computed.reads('searchTerm'),

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
