/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components|<%= classifiedModuleName %>',
};

export let <%= classifiedModuleName %> = () => {
  return {
    template: hbs`
      <h5 class="title is-5"><%= header %></h5>
      <<%= classifiedModuleName %>/>
    `,
    context: {},
  }
};
