import Component from '@ember/component';
import { computed as overridable } from 'ember-overridable-computed';
import { run } from '@ember/runloop';

const TAB = 9;
const ESC = 27;
const SPACE = 32;
const ARROW_UP = 38;
const ARROW_DOWN = 40;

export default Component.extend({
  classNames: ['dropdown'],

  options: overridable(() => []),
  selection: overridable(() => []),

  onSelect() {},

  isOpen: false,
  dropdown: null,

  capture(dropdown) {
    // It's not a good idea to grab a dropdown reference like this, but it's necessary
    // in order to invoke dropdown.actions.close in traverseList as well as
    // dropdown.actions.reposition when the label or selection length changes.
    this.set('dropdown', dropdown);
  },

  didReceiveAttrs() {
    const dropdown = this.dropdown;
    if (this.isOpen && dropdown) {
      run.scheduleOnce('afterRender', () => {
        dropdown.actions.reposition();
      });
    }
  },

  actions: {
    toggle({ key }) {
      const newSelection = this.selection.slice();
      if (newSelection.includes(key)) {
        newSelection.removeObject(key);
      } else {
        newSelection.addObject(key);
      }
      this.onSelect(newSelection);
    },

    openOnArrowDown(dropdown, e) {
      this.capture(dropdown);

      if (!this.isOpen && e.keyCode === ARROW_DOWN) {
        dropdown.actions.open(e);
        e.preventDefault();
      } else if (this.isOpen && (e.keyCode === TAB || e.keyCode === ARROW_DOWN)) {
        const optionsId = this.element.querySelector('.dropdown-trigger').getAttribute('aria-owns');
        const firstElement = document.querySelector(`#${optionsId} .dropdown-option`);

        if (firstElement) {
          firstElement.focus();
          e.preventDefault();
        }
      }
    },

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
    },
  },
});
