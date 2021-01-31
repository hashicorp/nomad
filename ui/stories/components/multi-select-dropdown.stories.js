import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components|Multi-Select Dropdown',
};

let options1 = [
  { key: 'option-1', label: 'Option One' },
  { key: 'option-2', label: 'Option Two' },
  { key: 'option-3', label: 'Option Three' },
  { key: 'option-4', label: 'Option Four' },
  { key: 'option-5', label: 'Option Five' },
];

let selection1 = ['option-2', 'option-4', 'option-5'];

export let Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Multi-Select Dropdown</h5>
      <MultiSelectDropdown
        @label="Example Dropdown"
        @options={{options1}}
        @selection={{selection1}}
        @onSelect={{action (mut selection1)}} />
      <p class="annotation">A wrapper around basic-dropdown for creating a list of checkboxes and tracking the state thereof.</p>
      `,
    context: {
      options1,
      selection1,
    },
  };
};

export let RightAligned = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Multi-Select Dropdown right-aligned</h5>
      <div style="display:flex; justify-content:flex-end">
        <MultiSelectDropdown
          @label="Example right-aligned Dropdown"
          @options={{options1}}
          @selection={{selection1}}
          @onSelect={{action (mut selection1)}} />
      </div>
      `,
    context: {
      options1,
      selection1,
    },
  };
};

export let ManyOptions = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Multi-Select Dropdown with many options</h5>
      <MultiSelectDropdown
        @label="Lots of options in here"
        @options={{optionsMany}}
        @selection={{selectionMany}}
        @onSelect={{action (mut selectionMany)}} />
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

export let Bar = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Multi-Select Dropdown bar</h5>
      <div class="button-bar">
        <MultiSelectDropdown
          @label="Datacenter"
          @options={{optionsDatacenter}}
          @selection={{selectionDatacenter}}
          @onSelect={{action (mut selectionDatacenter)}} />
        <MultiSelectDropdown
          @label="Type"
          @options={{optionsType}}
          @selection={{selectionType}}
          @onSelect={{action (mut selectionType)}} />
        <MultiSelectDropdown
          @label="Status"
          @options={{optionsStatus}}
          @selection={{selectionStatus}}
          @onSelect={{action (mut selectionStatus)}} />
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
      selectionDatacenter: ['jfk-1', 'jfk-2'],

      optionsType: [
        { key: 'batch', label: 'Batch' },
        { key: 'service', label: 'Service' },
        { key: 'system', label: 'System' },
        { key: 'periodic', label: 'Periodic' },
        { key: 'parameterized', label: 'Parameterized' },
      ],
      selectionType: ['system', 'service'],

      optionsStatus: [
        { key: 'pending', label: 'Pending' },
        { key: 'running', label: 'Running' },
        { key: 'dead', label: 'Dead' },
      ],
      selectionStatus: [],
    },
  };
};
