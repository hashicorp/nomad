/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { eq } from 'ember-truth-helpers';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import hdsClipboard from '@hashicorp/design-system-components/modifiers/hds-clipboard';

export default class CopyButton extends Component {
  @tracked state = null;
  resetTimerId = null;

  get text() {
    if (typeof this.args.clipboardText === 'function')
      return this.args.clipboardText;
    if (typeof this.args.clipboardText === 'string')
      return this.args.clipboardText;

    return String(this.args.clipboardText);
  }

  indicateSuccess = () => {
    this.state = 'success';

    if (this.resetTimerId) {
      clearTimeout(this.resetTimerId);
    }

    this.resetTimerId = setTimeout(() => {
      this.state = null;
      this.resetTimerId = null;
    }, 2000);
  };

  indicateError = () => {
    this.state = 'error';
  };

  willDestroy() {
    super.willDestroy(...arguments);

    if (this.resetTimerId) {
      clearTimeout(this.resetTimerId);
      this.resetTimerId = null;
    }
  }

  <template>
    <div class="copy-button {{if @inset 'inset'}}" ...attributes>
      {{#if (eq this.state "success")}}
        <div
          data-test-copy-success
          class="button is-small is-static
            {{if @compact 'is-compact'}}
            {{unless @border 'is-borderless'}}"
        >
          {{#if @inset}}
            <span aria-label="Copied!"><HdsIcon
                @name="clipboard-checked"
              /></span>
          {{else}}
            <span
              class="tooltip text-center always-active"
              role="tooltip"
              aria-label="Copied!"
            >
              <HdsIcon @name="clipboard-checked" />
            </span>
          {{/if}}
          {{yield}}
        </div>
      {{else if (eq this.state "error")}}
        <div
          class="button is-small is-static
            {{if @compact 'is-compact'}}
            {{unless @border 'is-borderless'}}"
        >
          {{#if @inset}}
            <span aria-label="Error copying"><HdsIcon
                @name="clipboard-x"
              /></span>
          {{else}}
            <span
              class="tooltip text-center"
              role="tooltip"
              aria-label="Error copying"
            >
              <HdsIcon @name="clipboard-x" />
            </span>
          {{/if}}
          {{yield}}
        </div>
      {{else}}
        <button
          type="button"
          title="Copy"
          data-clipboard-text={{this.text}}
          class="button is-small
            {{if @compact 'is-compact'}}
            {{unless @border 'is-borderless'}}
            {{if @inset 'is-inset'}}"
          {{hdsClipboard
            text=this.text
            onSuccess=this.indicateSuccess
            onError=this.indicateError
          }}
        >
          <HdsIcon @name="clipboard-copy" />
          {{yield}}
        </button>
      {{/if}}
    </div>
  </template>
}
