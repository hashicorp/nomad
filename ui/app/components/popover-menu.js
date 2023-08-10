/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { action } from '@ember/object';
import { scheduleOnce } from '@ember/runloop';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

const TAB = 9;
const ARROW_DOWN = 40;
const FOCUSABLE = [
  'a:not([disabled])',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'textarea:not([disabled])',
  '[tabindex]:not([disabled]):not([tabindex="-1"])',
].join(', ');

@classic
@classNames('popover')
export default class PopoverMenu extends Component {
  triggerClass = '';
  isOpen = false;
  isDisabled = false;
  label = '';

  dropdown = null;

  capture(dropdown) {
    // It's not a good idea to grab a dropdown reference like this, but it's necessary
    // in order to invoke dropdown.actions.close in traverseList as well as
    // dropdown.actions.reposition when the label or selection length changes.
    this.set('dropdown', dropdown);
  }

  didReceiveAttrs() {
    super.didReceiveAttrs();
    const dropdown = this.dropdown;
    if (this.isOpen && dropdown) {
      scheduleOnce('afterRender', this, this.repositionDropdown);
    }
  }

  repositionDropdown() {
    this.dropdown.actions.reposition();
  }

  @action
  openOnArrowDown(dropdown, e) {
    if (!this.isOpen && e.keyCode === ARROW_DOWN) {
      dropdown.actions.open(e);
      e.preventDefault();
    } else if (this.isOpen && (e.keyCode === TAB || e.keyCode === ARROW_DOWN)) {
      const optionsId = this.element
        .querySelector('.popover-trigger')
        .getAttribute('aria-owns');
      const popoverContentEl = document.querySelector(`#${optionsId}`);
      const firstFocusableElement = popoverContentEl.querySelector(FOCUSABLE);

      if (firstFocusableElement) {
        firstFocusableElement.focus();
        e.preventDefault();
      }
    }
  }
}
