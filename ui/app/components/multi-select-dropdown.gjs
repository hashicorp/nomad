/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { fn } from '@ember/helper';
import { tracked } from '@glimmer/tracking';
import { scheduleOnce } from '@ember/runloop';
import { on } from '@ember/modifier';
import { includes } from '@nullvoxpopuli/ember-composable-helpers';
import { didUpdate } from '@ember/render-modifiers';
import BasicDropdown from 'ember-basic-dropdown/components/basic-dropdown';

const TAB = 9;
const ESC = 27;
const SPACE = 32;
const ARROW_UP = 38;
const ARROW_DOWN = 40;

let dropdownInstanceCounter = 0;

export default class MultiSelectDropdown extends Component {
  @tracked isOpen = false;

  dropdown = null;
  triggerElement = null;

  labelElementId = `multi-select-dropdown-${dropdownInstanceCounter++}-label`;

  get options() {
    return this.args.options ?? [];
  }

  get selection() {
    return this.args.selection ?? [];
  }

  capture = (dropdown) => {
    this.dropdown = dropdown;
  };

  handleOpen = (dropdown) => {
    this.isOpen = true;
    this.capture(dropdown);
  };

  handleClose = () => {
    this.isOpen = false;
    this.dropdown = null;
  };

  captureTrigger = (element) => {
    this.triggerElement = element;
  };

  repositionDropdown = () => {
    if (this.isOpen && this.dropdown) {
      scheduleOnce('afterRender', this, this.repositionNow);
    }
  };

  repositionNow = () => {
    this.dropdown?.actions?.reposition?.();
  };

  toggle = ({ key }) => {
    const newSelection = [...this.selection];
    const index = newSelection.indexOf(key);

    if (index >= 0) {
      newSelection.splice(index, 1);
    } else {
      newSelection.push(key);
    }

    this.args.onSelect?.(newSelection);
  };

  openOnArrowDown = (dropdown, event) => {
    this.capture(dropdown);

    if (!this.isOpen && event.keyCode === ARROW_DOWN) {
      dropdown.actions.open(event);
      event.preventDefault();
    } else if (
      this.isOpen &&
      (event.keyCode === TAB || event.keyCode === ARROW_DOWN)
    ) {
      const optionsId = event.currentTarget?.getAttribute('aria-owns');
      const firstElement = optionsId
        ? document.querySelector(`#${optionsId} .dropdown-option`)
        : null;

      if (firstElement) {
        firstElement.focus();
        event.preventDefault();
      }
    }
  };

  traverseList = (option, event) => {
    if (event.keyCode === ESC) {
      const dropdown = this.dropdown;
      if (dropdown) {
        dropdown.actions.close(event);
        this.triggerElement?.focus?.();
        event.preventDefault();
        this.dropdown = null;
      }
    } else if (event.keyCode === ARROW_UP) {
      const prev = event.target.previousElementSibling;
      if (prev) {
        prev.focus();
        event.preventDefault();
      }
    } else if (event.keyCode === ARROW_DOWN) {
      const next = event.target.nextElementSibling;
      if (next) {
        next.focus();
        event.preventDefault();
      }
    } else if (event.keyCode === SPACE) {
      this.toggle(option);
      event.preventDefault();
    }
  };

  <template>
    <div
      class="dropdown"
      {{didUpdate this.repositionDropdown this.selection.length @label}}
      ...attributes
    >
      <BasicDropdown
        @ariaLabel="label-multi-select-dropdown"
        @ariaLabelledBy={{this.labelElementId}}
        @horizontalPosition="auto"
        @onOpen={{this.handleOpen}}
        @onClose={{this.handleClose}}
        as |dd|
      >
        <dd.Trigger
          data-test-dropdown-trigger
          class="dropdown-trigger"
          {{on "keyup" (fn this.openOnArrowDown dd)}}
          {{didUpdate this.captureTrigger}}
        >
          <div class="dropdown-trigger-label" id={{this.labelElementId}}>
            {{@label}}
            {{#if this.selection.length}}
              <span data-test-dropdown-count class="tag is-light">
                {{this.selection.length}}
              </span>
            {{/if}}
          </div>
          <span
            class="dropdown-trigger-icon ember-power-select-status-icon"
          ></span>
        </dd.Trigger>
        <dd.Content class="dropdown-options">
          {{#if this.options}}
            <ul
              role="listbox"
              aria-labelledby={{this.labelElementId}}
              data-test-dropdown-options
            >
              {{#each this.options key="key" as |option|}}
                <div
                  data-test-dropdown-option={{option.key}}
                  class="dropdown-option"
                  tabindex="0"
                  {{on "keyup" (fn this.traverseList option)}}
                  role="group"
                >
                  <label>
                    {{! template-lint-disable require-mandatory-role-attributes }}
                    {{! template-lint-disable require-context-role }}
                    <input
                      type="checkbox"
                      tabindex="-1"
                      checked={{includes option.key this.selection}}
                      role="option"
                      {{on "change" (fn this.toggle option)}}
                    />
                    {{option.label}}
                  </label>
                </div>
              {{/each}}
            </ul>
          {{else}}
            <ul
              aria-labelledby={{this.labelElementId}}
              data-test-dropdown-options
            >
              <li data-test-dropdown-empty class="dropdown-empty">
                No options
              </li>
            </ul>
          {{/if}}
        </dd.Content>
      </BasicDropdown>
    </div>
  </template>
}
