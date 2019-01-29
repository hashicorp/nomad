import Mixin from '@ember/object/mixin';
import { get, computed } from '@ember/object';
import { reads } from '@ember/object/computed';
import Fuse from 'fuse.js';

/**
  Searchable mixin

  Simple search filtering behavior for a list of objects.

  Properties to override:
    - searchTerm: the string to use as a query
    - searchProps: the props on each object to search
    -- exactMatchSearchProps: the props for exact search when props are different per search type
    -- regexSearchProps: the props for regex search when props are different per search type
    -- fuzzySearchProps: the props for fuzzy search when props are different per search type
    - exactMatchEnabled: (true) disable to not use the exact match search type
    - fuzzySearchEnabled: (false) enable to use the fuzzy search type
    - regexEnabled: (true) disable to disable the regex search type
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
  fuzzySearchProps: reads('searchProps'),

  // Three search modes
  exactMatchEnabled: true,
  fuzzySearchEnabled: false,
  regexEnabled: true,

  // Search should reset pagination. Not every instance of
  // search will be paired with pagination, but it's still
  // preferable to generalize this rather than risking it being
  // forgotten on a single page.
  resetPagination() {
    if (this.get('currentPage') != null) {
      this.set('currentPage', 1);
    }
  },

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

  listSearched: computed(
    'searchTerm',
    'listToSearch.[]',
    'exactMatchEnabled',
    'fuzzySearchEnabled',
    'regexEnabled',
    'exactMatchSearchProps.[]',
    'fuzzySearchProps.[]',
    'regexSearchProps.[]',
    function() {
      const searchTerm = this.get('searchTerm').trim();

      if (!searchTerm || !searchTerm.length) {
        return this.get('listToSearch');
      }

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
  ),
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
