/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { on } from '@ember/modifier';
import autofocus from 'nomad-ui/modifiers/autofocus';

export const MetadataEditor = <template>
  <form class="metadata-editor">
    <label>
      <strong>Key</strong>
      {{#if @constantKey}}
        <span class="constant-key">{{@kv.key}}</span>
      {{else}}
        <input
          {{autofocus}}
          id="new-meta-key"
          type="text"
          value={{@kv.key}}
          class="input"
          {{on "keyup" @onEdit}}
        />
      {{/if}}
    </label>
    <label>
      <strong>Value</strong>
      {{#if @autofocusValue}}
        <input
          data-test-metadata-editor-value
          type="text"
          value={{@kv.value}}
          class="input"
          {{autofocus}}
          {{on "keyup" @onEdit}}
        />
      {{else}}
        <input
          data-test-metadata-editor-value
          type="text"
          value={{@kv.value}}
          class="input"
          {{on "keyup" @onEdit}}
        />
      {{/if}}
    </label>
    <footer>
      {{yield}}
    </footer>
  </form>
</template>;

export default MetadataEditor;
