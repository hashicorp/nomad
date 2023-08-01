import { isPresent, text } from 'ember-cli-page-object';

export default (scope) => ({
  scope,

  svgIsPresent: isPresent('[data-test-gauge-svg]'),
  label: text('[data-test-label]'),
  percentage: text('[data-test-percentage]'),
});
