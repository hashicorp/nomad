import Mixin from '@ember/object/mixin';
import { get, computed } from '@ember/object';
import { reads } from '@ember/object/computed';
import Fuse from 'npm:fuse.js';

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
  exactMatchSearchProps: reads('searchProps'),
  regexSearchProps: reads('searchProps'),
  fuzzySearchProps: reads('searchprops'),

  // Three search modes
  exactMatchEnabled: true,
  fuzzySearchEnabled: false,
  regexEnabled: true,

  fuse: computed('listToSearch.[]', 'fuzzySearchProps.[]', function() {
    return new Fuse(this.get('listToSearch'), {
      shouldSort: true,
      threshold: 0.4,
      location: 0,
      distance: 100,
      tokenize: true,
      matchAllTokens: true,
      maxPatternLength: 32,
      minMatchCharLength: 1,
      keys: this.get('fuzzySearchProps') || [],
      getFn(item, key) {
        return get(item, key);
      },
    });
  }),

  listSearched: computed('searchTerm', 'listToSearch.[]', 'searchProps.[]', function() {
    const searchTerm = this.get('searchTerm');
    if (searchTerm && searchTerm.length) {
      const results = [];
      if (this.get('exactMatchEnabled')) {
        results.push(
          ...exactMatchSearch(
            searchTerm,
            this.get('listToSearch'),
            this.get('exactMatchSearchProps')
          )
        );
      }
      if (this.get('fuzzySearchEnabled')) {
        results.push(...this.get('fuse').search(searchTerm));
      }
      if (this.get('regexEnabled')) {
        results.push(
          ...regexSearch(searchTerm, this.get('listToSearch'), this.get('regexSearchProps'))
        );
      }
      return results.uniq();
    }
    return this.get('listToSearch');
  }),
});

function exactMatchSearch(term, list, keys) {
  if (term.length) {
    return list.filter(item => keys.some(key => get(item, key) === term));
  }
}

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
    return [];
  }
}
