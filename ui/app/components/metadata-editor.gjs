/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { on } from '@ember/modifier';
import { fn } from '@ember/helper';
import autofocus from 'nomad-ui/modifiers/autofocus';

const updateKVKey = (kv, event) => {
  if (kv) kv.key = event.target.value;
};

const updateKVValue = (kv, event) => {
  if (!kv) return;

  const value = event.target.value;
  if (typeof kv.setValue === 'function') {
    kv.setValue(value);
  }

  kv.value = value;
};

const preventSubmit = (event) => {
  event?.preventDefault?.();
};

export const MetadataEditor = <template>
  <form class="metadata-editor" {{on "submit" preventSubmit}} ...attributes>
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
          {{on "input" (fn updateKVKey @kv)}}
          {{on "input" @onEdit}}
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
          {{on "input" (fn updateKVValue @kv)}}
          {{on "input" @onEdit}}
          {{on "keyup" @onEdit}}
        />
      {{else}}
        <input
          data-test-metadata-editor-value
          type="text"
          value={{@kv.value}}
          class="input"
          {{on "input" (fn updateKVValue @kv)}}
          {{on "input" @onEdit}}
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
