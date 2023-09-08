/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components/Search Box',
};

export let Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Search Box</h5>
      <SearchBox
        @searchTerm={{mut searchTerm1}}
        @placeholder="Search things..." />
      <p class="annotation">The search box component is a thin wrapper around a simple input. Although the searchTerm passed to it is a mutable reference, internally search term is debounced. This is to prevent potentially expensive code that depends on searchTerm from recomputing many times as a user types.</p>
      <p class="annotation">There is no form of the search box component that defers updating the searchTerm reference until the user manually clicks a "Search" button. This can be achieved by placing a button next to the search bar component and using it to perform search, but search should be automatic whenever possible.</p>
      `,
  };
};

export let Compact = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Compact Search Box</h5>
      <SearchBox
        @searchTerm={{mut searchTerm2}}
        @placeholder="Search things..."
        @inputClass="is-compact" />
      <p class="annotation">Search box provides an inputClass property to control the inner input. This is nice for fitting the search box into smaller spaces, such as boxed-section heads.</p>
      `,
  };
};
