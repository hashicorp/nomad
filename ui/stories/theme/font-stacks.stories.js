import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Theme|Font Stacks',
};

export const FontStacks = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Font Stacks</h5>

      {{#each fontFamilies as |fontFamily|}}
        <h6 class="title is-6 with-headroom">{{fontFamily}}</h6>
        <FreestyleTypeface @fontFamily={{fontFamily}} />

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
      ],
    },
  };
};
