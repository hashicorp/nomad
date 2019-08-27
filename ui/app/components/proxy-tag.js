import Component from '@ember/component';

export default Component.extend({
  tagName: 'span',
  classNames: ['tag', 'is-secondary', 'is-structure'],

  'data-test-proxy-tag': true,
});
