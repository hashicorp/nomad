import Component from '@ember/component';

export default Component.extend({
  tagName: 'span',
  classNames: ['badge', 'is-light'],

  'data-test-proxy-tag': true,
});
