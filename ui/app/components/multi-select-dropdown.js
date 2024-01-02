/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { action } from '@ember/object';
import { computed as overridable } from 'ember-overridable-computed';
import { scheduleOnce } from '@ember/runloop';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

const TAB = 9;
const ESC = 27;
const SPACE = 32;
const ARROW_UP = 38;
const ARROW_DOWN = 40;

@classic
@classNames('dropdown')
export default class MultiSelectDropdown extends Component {
  @overridable(() => []) options;
  @overridable(() => []) selection;

  onSelect() {}

  isOpen = false;
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
  toggle({ key }) {
    const newSelection = this.selection.slice();
    if (newSelection.includes(key)) {
      newSelection.removeObject(key);
    } else {
      newSelection.addObject(key);
    }
    this.onSelect(newSelection);
  }

  @action
  openOnArrowDown(dropdown, e) {
    this.capture(dropdown);

    if (!this.isOpen && e.keyCode === ARROW_DOWN) {
      dropdown.actions.open(e);
      e.preventDefault();
    } else if (this.isOpen && (e.keyCode === TAB || e.keyCode === ARROW_DOWN)) {
      const optionsId = this.element
        .querySelector('.dropdown-trigger')
        .getAttribute('aria-owns');
      const firstElement = document.querySelector(
        `#${optionsId} .dropdown-option`
      );

      if (firstElement) {
        firstElement.focus();
        e.preventDefault();
      }
    }
  }

  @action
  traverseList(option, e) {
    if (e.keyCode === ESC) {
      // Close the dropdown
      const dropdown = this.dropdown;
      if (dropdown) {
        dropdown.actions.close(e);
        // Return focus to the trigger so tab works as expected
        const trigger = this.element.querySelector('.dropdown-trigger');
        if (trigger) trigger.focus();
        e.preventDefault();
        this.set('dropdown', null);
      }
    } else if (e.keyCode === ARROW_UP) {
      // previous item
      const prev = e.target.previousElementSibling;
      if (prev) {
        prev.focus();
        e.preventDefault();
      }
    } else if (e.keyCode === ARROW_DOWN) {
      // next item
      const next = e.target.nextElementSibling;
      if (next) {
        next.focus();
        e.preventDefault();
      }
    } else if (e.keyCode === SPACE) {
      this.send('toggle', option);
      e.preventDefault();
    }
  }
}
