import Component from '@ember/component';

export default Component.extend({
  classNames: ['accordion-head'],
  classNameBindings: ['isOpen::is-light', 'isExpandable::is-inactive'],

  'data-test-accordion-head': true,

  buttonLabel: 'toggle',
  isOpen: false,
  isExpandable: true,
  item: null,

  onClose() {},
  onOpen() {},
});
