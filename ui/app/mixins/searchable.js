import Ember from 'ember';

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

  listSearched: computed('searchTerm', 'listToSearch.[]', 'searchProps.[]', function() {
    const searchTerm = this.get('searchTerm');
    if (searchTerm && searchTerm.length) {
      return regexSearch(searchTerm, this.get('listToSearch'), this.get('searchProps'));
    }
    return this.get('listToSearch');
  }),
});

function regexSearch(term, list, keys) {
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
