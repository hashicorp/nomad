/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components/Proxy Tag',
};

export let ProxyTag = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Proxy Tag</h5>
      <h6 class="title is-6">Some kind of title <ProxyTag/></h6>
      `,
  };
};
