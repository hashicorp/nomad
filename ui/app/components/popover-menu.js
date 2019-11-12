import Component from '@ember/component';
import { run } from '@ember/runloop';

const TAB = 9;
// const ESC = 27;
const ARROW_DOWN = 40;
const FOCUSABLE = [
  'a:not([disabled])',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'textarea:not([disabled])',
  '[tabindex]:not([disabled]):not([tabindex="-1"])',
].join(', ');

export default Component.extend({
  classnames: ['popover'],

  triggerClass: '',

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
    openOnArrowDown(dropdown, e) {
      if (!this.isOpen && e.keyCode === ARROW_DOWN) {
        dropdown.actions.open(e);
        e.preventDefault();
      } else if (this.isOpen && (e.keyCode === TAB || e.keyCode === ARROW_DOWN)) {
        const optionsId = this.element.querySelector('.popover-trigger').getAttribute('aria-owns');
        const popoverContentEl = document.querySelector(`#${optionsId}`);
        const firstFocusableElement = popoverContentEl.querySelector(FOCUSABLE);

        if (firstFocusableElement) {
          firstFocusableElement.focus();
          e.preventDefault();
        }
      }
    },
  },
});
