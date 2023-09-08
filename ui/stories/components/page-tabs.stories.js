/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components/Page Tabs',
};

export let Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Page tabs</h5>
      <div class="tabs">
        <ul>
          <li><a href="javascript:;">Overview</a></li>
          <li><a href="javascript:;" class="is-active">Definition</a></li>
          <li><a href="javascript:;">Versions</a></li>
          <li><a href="javascript:;">Deployments</a></li>
        </ul>
      </div>
      `,
  };
};

export let Single = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Single page tab</h5>
      <div class="tabs">
        <ul>
          <li><a href="javascript:;" class="is-active">Overview</a></li>
        </ul>
      </div>
      `,
  };
};
