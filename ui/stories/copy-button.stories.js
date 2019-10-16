/* eslint-env node */
// FIXME Vault has an entry in .eslintignore to skip Storybook altogether…???
import hbs from 'htmlbars-inline-precompile';
import { storiesOf } from '@storybook/ember';
import notes from './copy-button.md';


storiesOf('CopyButton/', module)
  .addParameters({ options: { showPanel: true } })
  .add('CopyButton', () => ({
    template: hbs`
      <span class="tag is-hollow is-small no-text-transform">
        e8c898a0-794b-9063-7a7f-bf0c4a405f83
        {{copy-button clipboardText="e8c898a0-794b-9063-7a7f-bf0c4a405f83"}}
      </span>
    `,
    context: {},
  }),
  {notes}
  );
