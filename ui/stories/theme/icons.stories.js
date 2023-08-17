/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Theme/Icons',
};

export let Icons = () => ({
  template: hbs`
    <ul class="tile-list">
      {{#each (all-icons) as |icon|}}
        <li class="icon-tile">
          {{x-icon icon}}
          <code>{{icon}}</code>
        </li>
      {{/each}}
    </ul>
  `,
});
