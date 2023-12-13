/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components/Table, Configuration',
};

export let TableConfiguration = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Table, configuration</h5>
      <AttributesTable @attributePairs={{attributes}} @class="attributes-table" />
      `,
    context: {
      attributes: {
        key: 'val',
        deep: {
          key: 'val',
          more: 'stuff',
        },
        array: ['one', 'two', 'three', 'four'],
        very: {
          deep: {
            key: {
              incoming: {
                one: 1,
                two: 2,
                three: 3,
                four: 'surprisingly long value that is unlike the other properties in this object',
              },
            },
          },
        },
      },
    },
  };
};

TableConfiguration.story = {
  title: 'Table, Configuration',
};
