import { isPresent, clickable, collection, text, attribute } from 'ember-cli-page-object';

export default scope => ({
  scope,

  isPresent: isPresent(),

  toggle: clickable('[data-test-dropdown-trigger]'),

  options: collection('[data-test-dropdown-option]', {
    testContainer: '#ember-testing',
    resetScope: true,
    label: text(),
    key: attribute('data-test-dropdown-option'),
    toggle: clickable('label'),
  }),
});
