/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { on } from '@ember/modifier';
import { HdsButton } from '@hashicorp/design-system-components/components';
import onClickOutside from 'ember-click-outside/modifiers/on-click-outside';

const noOp = () => {};

export default class TwoStepButton extends Component {
  @tracked state = 'idle';

  get isIdle() {
    return this.state === 'idle';
  }

  get isPendingConfirmation() {
    return this.state === 'prompt';
  }

  get idleText() {
    return this.args.idleText ?? '';
  }

  get cancelText() {
    return this.args.cancelText ?? '';
  }

  get confirmText() {
    return this.args.confirmText ?? '';
  }

  get confirmationMessage() {
    return this.args.confirmationMessage ?? '';
  }

  get awaitingConfirmation() {
    return this.args.awaitingConfirmation ?? false;
  }

  get disabled() {
    return this.args.disabled ?? false;
  }

  get alignRight() {
    return this.args.alignRight ?? false;
  }

  get inlineText() {
    return this.args.inlineText ?? false;
  }

  get title() {
    return this.args.title ?? '';
  }

  get onConfirm() {
    return this.args.onConfirm ?? noOp;
  }

  get onCancel() {
    return this.args.onCancel ?? noOp;
  }

  get onPrompt() {
    return this.args.onPrompt ?? noOp;
  }

  get rootClass() {
    const classes = ['two-step-button'];

    if (this.inlineText) {
      classes.push('has-inline-text');
    }

    if (this.args.fadingBackground) {
      classes.push('has-fading-background');
    }

    return classes.join(' ');
  }

  setToIdle = () => {
    this.state = 'idle';
  };

  promptForConfirmation = () => {
    this.onPrompt();
    this.state = 'prompt';
  };

  cancel = () => {
    this.setToIdle();
    this.onCancel();
  };

  confirm = () => {
    this.setToIdle();
    this.onConfirm();
  };

  handleOutsideClick = () => {
    if (this.isPendingConfirmation && !this.awaitingConfirmation) {
      this.onCancel();
      this.setToIdle();
    }
  };

  <template>
    <div
      class={{this.rootClass}}
      ...attributes
      {{onClickOutside this.handleOutsideClick capture=true}}
    >
      {{#if this.isIdle}}
        <HdsButton
          data-test-idle-button
          @size={{@size}}
          @text={{this.idleText}}
          @color="critical"
          disabled={{this.disabled}}
          title={{this.title}}
          {{on "click" this.promptForConfirmation}}
        />
      {{else if this.isPendingConfirmation}}
        <span
          data-test-confirmation-message
          class="confirmation-text
            {{if this.alignRight 'is-right-aligned'}}
            {{if this.inlineText 'has-text-inline'}}"
        >
          {{this.confirmationMessage}}
        </span>
        <HdsButton
          data-test-cancel-button
          @size={{@size}}
          @text={{this.cancelText}}
          @color="secondary"
          class="is-inline"
          disabled={{this.awaitingConfirmation}}
          {{on "click" this.cancel}}
        />
        <HdsButton
          data-test-confirm-button
          @size={{@size}}
          @text={{if this.awaitingConfirmation "Loading..." this.confirmText}}
          @color="critical"
          class="is-inline"
          disabled={{this.awaitingConfirmation}}
          {{on "click" this.confirm}}
        />
      {{/if}}
    </div>
  </template>
}
