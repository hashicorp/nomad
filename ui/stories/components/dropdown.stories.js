/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components/Dropdown',
};

let options = [
  { name: 'Consul' },
  { name: 'Nomad' },
  { name: 'Packer' },
  { name: 'Terraform' },
  { name: 'Vagrant' },
  { name: 'Vault' },
];

export let Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Dropdown</h5>
      <PowerSelect @options={{options}} @selected={{selectedOption}} @searchField="name" @searchEnabled={{gt options.length 10}} @onChange={{action (mut selectedOption)}} as |option|>
        {{option.name}}
      </PowerSelect>
      <p class="annotation">Power Select currently fulfills all of Nomad's dropdown needs out of the box.</p>
      `,
    context: {
      options,
    },
  };
};

export let Resized = () => {
  return {
    template: hbs`
    <h5 class="title is-5">Dropdown resized</h5>
    <div class="columns">
      <div class="column is-3">
        <PowerSelect @options={{options}} @selected={{selectedOption2}} @searchField="name" @searchEnabled={{gt options.length 10}} @onChange={{action (mut selectedOption2)}} as |option|>
          {{option.name}}
        </PowerSelect>
      </div>
    </div>
    <p class="annotation">Dropdowns are always 100% wide. To control the width of a dropdown, adjust the dimensions of its container. One way to achieve this is using columns.</p>
    `,
    context: {
      options,
    },
  };
};

export let Search = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Dropdown with search</h5>
      <div class="columns">
        <div class="column is-3">
          <PowerSelect @options={{manyOptions}} @selected={{selectedOption3}} @searchField="name" @searchEnabled={{gt manyOptions.length 10}} @onChange={{action (mut selectedOption3)}} as |option|>
            {{option.name}}
          </PowerSelect>
        </div>
      </div>
      <p class="annotation">Whether or not the dropdown has a search box is configurable. Typically the default is to show a search once a dropdown has more than 10 options.</p>
      `,
    context: {
      manyOptions: [
        'One',
        'Two',
        'Three',
        'Four',
        'Five',
        'Six',
        'Seven',
        'Eight',
        'Nine',
        'Ten',
        'Eleven',
        'Twelve',
        'Thirteen',
        'Fourteen',
        'Fifteen',
      ].map((name) => ({ name })),
    },
  };
};
