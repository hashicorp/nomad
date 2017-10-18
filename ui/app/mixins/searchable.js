import Ember from 'ember';
import Fuse from 'npm:fuse.js';

const { Mixin, computed, get } = Ember;

/**
  Searchable mixin

  Simple search filtering behavior for a list of objects.

  Properties to override:
    - searchTerm: the string to use as a query
    - searchProps: the props on each object to search
    - listToSearch: the list of objects to search

  Properties provided:
    - listSearched: a subset of listToSearch of items that meet the search criteria
*/
export default Mixin.create({
  searchTerm: '',
  listToSearch: computed(() => []),
  searchProps: null,

  fuse: computed('listToSearch.[]', 'searchProps.[]', function() {
    return new Fuse(this.get('listToSearch'), {
      shouldSort: true,
      threshold: 0.6,
      location: 0,
      distance: 100,
      maxPatternLength: 32,
      minMatchCharLength: 1,
      keys: this.get('searchProps') || [],
      getFn(item, key) {
        return get(item, key);
      },
    });
  }),

  listSearched: computed('fuse', 'searchTerm', function() {
    const { fuse, searchTerm } = this.getProperties('fuse', 'searchTerm');
    if (searchTerm && searchTerm.length) {
      return regexSearch(searchTerm, fuse);
    }
    return this.get('listToSearch');
  }),
});

function regexSearch(term, { list, options: { keys } }) {
  if (term.length) {
    try {
      const regex = new RegExp(term, 'i');
      // Test the value of each key for each object against the regex
      // All that match are returned.
      return list.filter(item => keys.some(key => regex.test(get(item, key))));
    } catch (e) {
      // Swallow the error; most likely due to an eager search of an incomplete regex
    }
  }
}
