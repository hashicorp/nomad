/* eslint-env node */
// FIXME Vault has an entry in .eslintignore to skip Storybook altogether…???
import hbs from 'htmlbars-inline-precompile';
import { storiesOf } from '@storybook/ember';
import notes from './copy-button.md';


storiesOf('CopyButton/', module)
  .addParameters({ options: { showPanel: true } })
  .add('CopyButton', () => ({
    template: hbs`
      <h5 class="title is-5">Copy Button</h5>
      <CopyButton/>
    `,
    context: {},
  }),
  {notes}
  );
