/* eslint-env node */
// FIXME Vault has an entry in .eslintignore to skip Storybook altogether…???
import hbs from 'htmlbars-inline-precompile';
import { withKnobs, text } from '@storybook/addon-knobs';
import notes from './copy-button.md';

export default {
  title: 'CopyButton',
  decorators: [withKnobs],
  parameters: {
    notes,
  },
};

export const CopyButton = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Copy Button</h5>
      <span class="tag is-hollow is-small no-text-transform">
        {{clipboardText}}
        <CopyButton @clipboardText={{clipboardText}} />
      </span>
    `,
    context: {
      clipboardText: text('Clipboard Text', 'e8c898a0-794b-9063-7a7f-bf0c4a405f83'),
    },
  };
};

CopyButton.story = {
  name: 'CopyButton',
};
