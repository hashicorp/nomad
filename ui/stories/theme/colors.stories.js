/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';
import { htmlSafe } from '@ember/string';

export default {
  title: 'Theme/Colors',
};

export let Colors = () => {
  return {
    template: hbs`
      {{#each palettes as |palette|}}
        <div class='palette'>
          <div class='title'>{{palette.title}}</div>
          <div class='description'>{{palette.description}}</div>
          {{#each palette.colors as |color|}}
            <div class='item'>
              <div class='color' style={{color.style}}></div>
              <div class='info'>
                <p class='hex'>{{color.base}}</p>
                <p class='name'>{{color.name}}</p>
              </div>
            </div>
          {{/each}}
        </div>
      {{/each}}
      `,
    context: {
      palettes: [
        {
          title: 'Nomad Theme',
          description: 'Accent and neutrals.',
          colors: [
            {
              name: 'Primary',
              base: '#25ba81',
            },
            {
              name: 'Primary Dark',
              base: '#1d9467',
            },
            {
              name: 'Text',
              base: '#0a0a0a',
            },
            {
              name: 'Link',
              base: '#1563ff',
            },
            {
              name: 'Gray',
              base: '#bbc4d1',
            },
            {
              name: 'Off-white',
              base: '#f5f5f5',
            },
          ],
        },
        {
          title: 'Product Colors',
          description:
            'Colors from other HashiCorp products. Often borrowed for alternative accents and color schemes.',
          colors: [
            {
              name: 'Consul Pink',
              base: '#ff0087',
            },
            {
              name: 'Consul Pink Dark',
              base: '#c62a71',
            },
            {
              name: 'Packer Blue',
              base: '#1daeff',
            },
            {
              name: 'Packer Blue Dark',
              base: '#1d94dd',
            },
            {
              name: 'Terraform Purple',
              base: '#5c4ee5',
            },
            {
              name: 'Terraform Purple Dark',
              base: '#4040b2',
            },
            {
              name: 'Vagrant Blue',
              base: '#1563ff',
            },
            {
              name: 'Vagrant Blue Dark',
              base: '#104eb2',
            },
            {
              name: 'Nomad Green',
              base: '#25ba81',
            },
            {
              name: 'Nomad Green Dark',
              base: '#1d9467',
            },
            {
              name: 'Nomad Green Darker',
              base: '#16704d',
            },
          ],
        },
        {
          title: 'Emotive Colors',
          description: 'Colors used in conjunction with an emotional response.',
          colors: [
            {
              name: 'Success',
              base: '#23d160',
            },
            {
              name: 'Warning',
              base: '#fa8e23',
            },
            {
              name: 'Danger',
              base: '#c84034',
            },
            {
              name: 'Info',
              base: '#1563ff',
            },
          ],
        },
      ].map((palette) => {
        palette.colors.forEach((color) => {
          color.style = htmlSafe(`background-color: ${color.base}`);
        });
        return palette;
      }),
    },
  };
};
