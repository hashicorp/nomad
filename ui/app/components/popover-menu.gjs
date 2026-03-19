/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { fn, concat } from '@ember/helper';
import { on } from '@ember/modifier';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import BasicDropdown from 'ember-basic-dropdown/components/basic-dropdown';

const TAB = 9;
const ARROW_DOWN = 40;
const FOCUSABLE = [
  'a:not([disabled])',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'textarea:not([disabled])',
  '[tabindex]:not([disabled]):not([tabindex="-1"])',
].join(', ');

export default class PopoverMenu extends Component {
  @tracked isOpen = false;

  dropdown = null;

  get triggerClass() {
    return this.args.triggerClass || '';
  }

  get label() {
    return this.args.label || '';
  }

  get isDisabled() {
    return this.args.isDisabled || false;
  }

  get tooltip() {
    return this.args.tooltip;
  }

  capture = (dropdown) => {
    // A direct dropdown reference is required for close/reposition controls.
    this.dropdown = dropdown;
  };

  handleOpen = (dropdown) => {
    this.isOpen = true;
    this.capture(dropdown);
  };

  handleClose = () => {
    this.isOpen = false;
  };

  openOnArrowDown = (dropdown, event) => {
    if (!this.isOpen && event.keyCode === ARROW_DOWN) {
      dropdown.actions.open(event);
      event.preventDefault();
      return;
    }

    if (
      !this.isOpen ||
      (event.keyCode !== TAB && event.keyCode !== ARROW_DOWN)
    ) {
      return;
    }

    const optionsId = event.currentTarget?.getAttribute('aria-owns');
    if (!optionsId) return;

    const popoverContentEl = document.querySelector(`#${optionsId}`);
    const firstFocusableElement = popoverContentEl?.querySelector(FOCUSABLE);

    if (firstFocusableElement) {
      firstFocusableElement.focus();
      event.preventDefault();
    }
  };

  <template>
    <div class="popover" ...attributes>
      <BasicDropdown
        @ariaLabel="label-popover-menu"
        @ariaLabelledBy="label-popover-menu"
        @horizontalPosition="right"
        @disabled={{this.isDisabled}}
        @onOpen={{this.handleOpen}}
        @onClose={{this.handleClose}}
        as |dd|
      >
        {{#let dd.Trigger dd.Content as |Trigger Content|}}
          <Trigger
            data-test-popover-trigger
            class={{concat
              "popover-trigger button is-primary "
              this.triggerClass
              (if this.isDisabled " is-disabled")
            }}
            aria-label={{this.tooltip}}
            {{on "keyup" (fn this.openOnArrowDown dd)}}
          >
            {{this.label}}
            <HdsIcon @name="chevron-down" @isInline={{true}} />
          </Trigger>
          <Content data-test-popover-menu class="popover-content">
            {{yield dd}}
          </Content>
        {{/let}}
      </BasicDropdown>
    </div>
  </template>
}
