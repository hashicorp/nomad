import { clickable, collection, hasClass, text } from 'ember-cli-page-object';

export default {
  scope: '[data-test-lifecycle-chart]',

  title: text('.boxed-section-head'),

  phases: collection('[data-test-lifecycle-phase]', {
    name: text('[data-test-name]'),

    isActive: hasClass('is-active'),
  }),

  tasks: collection('[data-test-lifecycle-task]', {
    name: text('[data-test-name]'),
    lifecycle: text('[data-test-lifecycle]'),

    isActive: hasClass('is-active'),
    isFinished: hasClass('is-finished'),

    isMain: hasClass('main'),
    isPrestart: hasClass('prestart'),
    isSidecar: hasClass('sidecar'),
    isPoststop: hasClass('poststop'),

    visit: clickable('a'),
  }),
};
