/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';
import { htmlSafe } from '@ember/string';

export default {
  title: 'Theme/Font Stacks',
};

export let FontStacks = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Font Stacks</h5>

      {{#each fontFamilies as |fontFamily|}}
        <h6 class="title is-6 with-headroom">{{fontFamily.name}}</h6>
        <div class="typeface" style={{fontFamily.style}}>
          <div class="hero">Aa</div>
          <p class="sample">A B C D E F G H I J K L M N O P Q R S T U V W X Y Z</p>
          <p class="sample">a b c d e f g h i j k l m n o p q r s t u v w x y z</p>
          <p class="sample">0 1 2 3 4 5 6 7 8 9</p>
        </div>
        <br>
      {{/each}}
      `,
    context: {
      fontFamilies: [
        '-apple-system',
        'BlinkMacSystemFont',
        'Segoe UI',
        'Roboto',
        'Oxygen-Sans',
        'Ubuntu',
        'Cantarell',
        'Helvetica Neue',
        'sans-serif',
        'monospace',
      ].map((family) => {
        return {
          name: family,
          style: htmlSafe(`font-family: ${family}`),
        };
      }),
    },
  };
};
