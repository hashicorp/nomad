import Component from '@ember/component';

export default Component.extend({
  tagName: 'label',
  classNames: ['toggle'],
  classNameBindings: ['isDisabled:is-disabled', 'isActive:is-active'],

  onToggle() {},
});
