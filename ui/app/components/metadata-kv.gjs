/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { on } from '@ember/modifier';
import CopyButton from 'nomad-ui/components/copy-button';
import MetadataEditor from 'nomad-ui/components/metadata-editor';
import { hash } from '@ember/helper';
import { not, or } from 'ember-truth-helpers';

export default class MetadataKv extends Component {
  @tracked editing = false;
  // eslint-disable-next-line ember/no-tracked-properties-from-args
  @tracked value = this.args.value;

  get prefixedKey() {
    return this.args.prefix
      ? `${this.args.prefix}.${this.args.key}`
      : this.args.key;
  }

  onEdit = (event) => {
    if (event.key === 'Escape') {
      this.cancelEditing();
    }
  };

  setValue = (value) => {
    this.value = value;
  };

  startEditing = () => {
    this.editing = true;
  };

  cancelEditing = () => {
    this.editing = false;
    this.value = this.args.value;
  };

  saveMetadata = () => {
    this.args.onKVSave?.({ key: this.prefixedKey, value: this.value });
    this.editing = false;
  };

  deleteMetadata = () => {
    this.args.onKVSave?.({ key: this.prefixedKey, value: null });
    this.editing = false;
  };

  <template>
    <tr data-test-attributes-section>
      {{#if this.editing}}
        <td colspan="2">
          <MetadataEditor
            @kv={{hash
              key=this.prefixedKey
              value=this.value
              setValue=this.setValue
            }}
            @onEdit={{this.onEdit}}
            @constantKey={{true}}
            @autofocusValue={{true}}
          >
            <button
              data-test-update-metadata
              disabled={{or (not @key) (not this.value)}}
              type="submit"
              class="button is-primary"
              {{on "click" this.saveMetadata}}
            >
              Update
              {{this.prefixedKey}}
            </button>
            <button
              data-test-delete-metadata
              type="button"
              class="button is-danger"
              {{on "click" this.deleteMetadata}}
            >
              Delete Property
            </button>
            <button
              data-test-cancel-metadata
              type="button"
              class="button"
              {{on "click" this.cancelEditing}}
            >
              Cancel
            </button>
          </MetadataEditor>
        </td>
      {{else}}
        <td data-test-key>
          {{#if @prefix}}<span
              class="is-faded"
              data-test-prefix
            >{{@prefix}}.</span>{{/if}}
          {{~@key}}
        </td>
        <td title="{{@value}}">
          <CopyButton
            @inset={{true}}
            @compact={{true}}
            @clipboardText={{@value}}
          />
          <span data-test-value>{{@value}}</span>
          {{#if @editable}}
            <button
              class="button is-light is-compact edit-existing-metadata-button"
              type="button"
              {{on "click" this.startEditing}}
            >Edit</button>
          {{/if}}
        </td>
      {{/if}}
    </tr>
  </template>
}
