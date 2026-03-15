/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

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
// eslint-disable-next-line ember/no-new-mixins
export default Mixin.create({
  searchTerm: '',
  listToSearch: computed(function () {
    return [];
  }),

  searchProps: null,
  exactMatchSearchProps: reads('searchProps'),
  regexSearchProps: reads('searchProps'),
  fuzzySearchProps: reads('searchProps'),

  // Three search modes
  exactMatchEnabled: true,
  fuzzySearchEnabled: false,
  includeFuzzySearchMatches: false,
  regexEnabled: true,

  // Search should reset pagination. Not every instance of
  // search will be paired with pagination, but it's still
  // preferable to generalize this rather than risking it being
  // forgotten on a single page.
  resetPagination() {
    if (this.currentPage != null) {
      this.set('currentPage', 1);
    }
  },

  fuse: computed(
    'fuzzySearchProps.[]',
    'includeFuzzySearchMatches',
    'listToSearch.[]',
    function () {
      return new Fuse(this.listToSearch, {
        shouldSort: true,
        threshold: 0.4,
        distance: 100,
        ignoreLocation: false,
        minMatchCharLength: 1,
        includeMatches: this.includeFuzzySearchMatches,
        keys: this.fuzzySearchProps || [],
        getFn(item, path) {
          const normalizedPath = Array.isArray(path) ? path.join('.') : path;

          if (
            typeof normalizedPath === 'string' ||
            typeof normalizedPath === 'number'
          ) {
            return get(item, normalizedPath);
          }

          return undefined;
        },
      });
    }
  ),

  listSearched: computed(
    'exactMatchEnabled',
    'exactMatchSearchProps.[]',
    'fuse',
    'fuzzySearchEnabled',
    'fuzzySearchProps.[]',
    'includeFuzzySearchMatches',
    'listToSearch.[]',
    'regexEnabled',
    'regexSearchProps.[]',
    'searchTerm',
    function () {
      const searchTerm = this.searchTerm.trim();

      if (!searchTerm || !searchTerm.length) {
        return this.listToSearch;
      }

      const results = [];

      if (this.exactMatchEnabled) {
        results.push(
          ...exactMatchSearch(
            searchTerm,
            this.listToSearch,
            this.exactMatchSearchProps
          )
        );
      }

      if (this.fuzzySearchEnabled) {
        let fuseSearchResults = fuzzySearch(searchTerm, this.fuse);

        if (this.includeFuzzySearchMatches) {
          fuseSearchResults = fuseSearchResults.map((result) => {
            const item = result.item;
            if (
              item &&
              typeof item.set === 'function' &&
              !isDestroyedRecord(item)
            ) {
              item.set('fuzzySearchMatches', result.matches || []);
            }
            return item;
          });
        } else {
          fuseSearchResults = fuseSearchResults.map((result) => result.item);
        }

        results.push(...fuseSearchResults);
      }

      if (this.regexEnabled) {
        results.push(
          ...regexSearch(searchTerm, this.listToSearch, this.regexSearchProps)
        );
      }

      return results.filter((item) => !isDestroyedRecord(item)).uniq();
    }
  ),
});

function isDestroyedRecord(record) {
  return Boolean(
    record?.isDestroying || record?.isDestroyed || record?.destroyed,
  );
}

function exactMatchSearch(term, list, keys) {
  if (term.length) {
    return list.filter((item) => keys.some((key) => get(item, key) === term));
  }
}

function regexSearch(term, list, keys) {
  if (term.length) {
    try {
      const regex = new RegExp(term, 'i');
      // Test the value of each key for each object against the regex
      // All that match are returned.
      return list.filter((item) =>
        keys.some((key) => regex.test(get(item, key)))
      );
    } catch (e) {
      // Swallow the error; most likely due to an eager search of an incomplete regex
    }
    return [];
  }
}

function fuzzySearch(term, fuse) {
  const tokens = term.split(/\s+/).filter(Boolean);
  if (!tokens.length) {
    return [];
  }

  const firstTokenResults = fuse.search(tokens[0]);
  if (tokens.length === 1) {
    return firstTokenResults;
  }

  const tokenMatchesByItem = new Map();

  firstTokenResults.forEach((result) => {
    tokenMatchesByItem.set(result.item, {
      result,
      tokenCount: 1,
      matches: result.matches || [],
    });
  });

  for (let i = 1; i < tokens.length; i++) {
    const tokenResults = fuse.search(tokens[i]);
    const itemsForToken = new Set(tokenResults.map((result) => result.item));

    tokenResults.forEach((result) => {
      const entry = tokenMatchesByItem.get(result.item);
      if (entry) {
        entry.tokenCount++;
        if (result.matches?.length) {
          entry.matches.push(...result.matches);
        }
      }
    });

    tokenMatchesByItem.forEach((entry, item) => {
      if (!itemsForToken.has(item)) {
        tokenMatchesByItem.delete(item);
      }
    });
  }

  return firstTokenResults
    .filter((result) => {
      const entry = tokenMatchesByItem.get(result.item);
      return entry && entry.tokenCount === tokens.length;
    })
    .map((result) => {
      const entry = tokenMatchesByItem.get(result.item);
      return {
        ...result,
        matches: entry?.matches || result.matches || [],
      };
    });
}
