/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Theme/Text Sizing',
};

export let TextSizing = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Text sizing</h5>
      <div class="block">
        <h1 class="title">Large Title</h1>
        <p>Some prose to follow the large title. Not necessarily meant for reading.</p>
      </div>
      <div class="block">
        <h2 class="title is-4">Medium Title</h2>
        <p>Some prose to follow the large title. Not necessarily meant for reading.</p>
      </div>
      <div class="block">
        <h3 class="title is-5">Small Title</h3>
        <p>Some prose to follow the large title. Not necessarily meant for reading.</p>
      </div>
      `,
  };
};
