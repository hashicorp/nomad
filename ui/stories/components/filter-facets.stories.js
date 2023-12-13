/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components/Filter Facets',
};

let options1 = [
  { key: 'option-1', label: 'Option One' },
  { key: 'option-2', label: 'Option Two' },
  { key: 'option-3', label: 'Option Three' },
  { key: 'option-4', label: 'Option Four' },
  { key: 'option-5', label: 'Option Five' },
];

let selection1 = ['option-2', 'option-4', 'option-5'];

export let MultiSelect = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Multi-Select Dropdown</h5>
      <MultiSelectDropdown
        @label="Example Dropdown"
        @options={{this.options1}}
        @selection={{this.selection1}}
        @onSelect={{action (mut selection1)}} />
      <p class="annotation">A wrapper around basic-dropdown for creating a list of checkboxes and tracking the state thereof.</p>
      `,
    context: {
      options1,
      selection1,
    },
  };
};

export let SingleSelect = () => ({
  template: hbs`
    <h5 class="title is-5">Single-Select Dropdown</h5>
    <SingleSelectDropdown
      @label="Single"
      @options={{this.options1}}
      @selection={{this.selection}}
      @onSelect={{action (mut this.selection)}} />
  `,
  context: {
    options1,
    selection: 'option-2',
  },
});

export let RightAligned = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Multi-Select Dropdown right-aligned</h5>
      <div style="display:flex; justify-content:flex-end">
        <MultiSelectDropdown
          @label="Example right-aligned Dropdown"
          @options={{this.options1}}
          @selection={{this.selection1}}
          @onSelect={{action (mut selection1)}} />
      </div>
      `,
    context: {
      options1,
      selection1,
    },
  };
};

export let ManyOptionsMulti = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Multi-Select Dropdown with many options</h5>
      <MultiSelectDropdown
        @label="Lots of options in here"
        @options={{this.optionsMany}}
        @selection={{this.selectionMany}}
        @onSelect={{action (mut this.selectionMany)}} />
      <p class="annotation">
        A strength of the multi-select-dropdown is its simple presentation. It is quick to select options and it is quick to remove options.
        However, this strength becomes a weakness when there are too many options. Since the selection isn't pinned in any way, removing a selection
        can become an adventure of scrolling up and down. Also since the selection isn't pinned, this component can't support search, since search would
        entirely mask the selection.
      </p>
      `,
    context: {
      optionsMany: Array(100)
        .fill(null)
        .map((_, i) => ({ label: `Option ${i}`, key: `option-${i}` })),
      selectionMany: [],
    },
  };
};

export let ManyOptionsSingle = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Single-Select Dropdown with many options</h5>
      <SingleSelectDropdown
        @label="Lots of options in here"
        @options={{this.optionsMany}}
        @selection={{this.selection}}
        @onSelect={{action (mut this.selection)}} />
      <p class="annotation">
        Single select supports search at a certain option threshold via Ember Power Select.
      </p>
      `,
    context: {
      optionsMany: Array(100)
        .fill(null)
        .map((_, i) => ({ label: `Option ${i}`, key: `option-${i}` })),
      selection: 'option-1',
    },
  };
};

export let Bar = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Multi-Select Dropdown bar</h5>
      <div class="button-bar">
        <MultiSelectDropdown
          @label="Datacenter"
          @options={{this.optionsDatacenter}}
          @selection={{this.selectionDatacenter}}
          @onSelect={{action (mut this.selectionDatacenter)}} />
        <MultiSelectDropdown
          @label="Type"
          @options={{this.optionsType}}
          @selection={{this.selectionType}}
          @onSelect={{action (mut this.selectionType)}} />
        <MultiSelectDropdown
          @label="Status"
          @options={{this.optionsStatus}}
          @selection={{this.selectionStatus}}
          @onSelect={{action (mut this.selectionStatus)}} />
      </div>
      <h5 class="title is-5">Single-Select Dropdown bar</h5>
      <div class="button-bar">
        <SingleSelectDropdown
          @label="Datacenter"
          @options={{this.optionsDatacenter}}
          @selection={{this.selectionDatacenterSingle}}
          @onSelect={{action (mut this.selectionDatacenterSingle)}} />
        <SingleSelectDropdown
          @label="Type"
          @options={{this.optionsType}}
          @selection={{this.selectionTypeSingle}}
          @onSelect={{action (mut this.selectionTypeSingle)}} />
        <SingleSelectDropdown
          @label="Status"
          @options={{this.optionsStatus}}
          @selection={{this.selectionStatusSingle}}
          @onSelect={{action (mut this.selectionStatusSingle)}} />
      </div>
      <h5 class="title is-5">Mixed Dropdown bar</h5>
      <div class="button-bar">
        <SingleSelectDropdown
          @label="Datacenter"
          @options={{this.optionsDatacenter}}
          @selection={{this.selectionDatacenterSingle}}
          @onSelect={{action (mut this.selectionDatacenterSingle)}} />
        <MultiSelectDropdown
          @label="Type"
          @options={{this.optionsType}}
          @selection={{this.selectionType}}
          @onSelect={{action (mut this.selectionType)}} />
        <MultiSelectDropdown
          @label="Status"
          @options={{this.optionsStatus}}
          @selection={{this.selectionStatus}}
          @onSelect={{action (mut this.selectionStatus)}} />
      </div>
      <p class="annotation">
        Since this is a core component for faceted search, it makes sense to letruct an arrangement of multi-select dropdowns.
        Do this by wrapping all the options in a <code>.button-bar</code> container.
      </p>
      `,
    context: {
      optionsDatacenter: [
        { key: 'pdx-1', label: 'pdx-1' },
        { key: 'jfk-1', label: 'jfk-1' },
        { key: 'jfk-2', label: 'jfk-2' },
        { key: 'muc-1', label: 'muc-1' },
      ],
      selectionDatacenter: ['jfk-1'],
      selectionDatacenterSingle: 'jfk-1',

      optionsType: [
        { key: 'batch', label: 'Batch' },
        { key: 'service', label: 'Service' },
        { key: 'system', label: 'System' },
        { key: 'periodic', label: 'Periodic' },
        { key: 'parameterized', label: 'Parameterized' },
      ],
      selectionType: ['system', 'service'],
      selectionTypeSingle: 'system',

      optionsStatus: [
        { key: 'pending', label: 'Pending' },
        { key: 'running', label: 'Running' },
        { key: 'dead', label: 'Dead' },
      ],
      selectionStatus: [],
      selectionStatusSingle: 'dead',
    },
  };
};
