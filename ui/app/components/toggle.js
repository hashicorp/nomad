import Component from '@ember/component';

const SPACE = 32;

export default Component.extend({
  tagName: 'label',
  classNames: ['toggle'],
  classNameBindings: ['isDisabled:is-disabled', 'isActive:is-active'],

  'data-test-label': true,

  isActive: false,
  isDisabled: false,
  onToggle() {},
  onSpacebar() {},

  onKeypress(e) {
    if (e.keyCode === SPACE) {
      this.onSpacebar(e);
    }
  },
});
