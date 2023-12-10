/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

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
    isPrestartEphemeral: hasClass('prestart-ephemeral'),
    isPrestartSidecar: hasClass('prestart-sidecar'),
    isPoststartEphemeral: hasClass('poststart-ephemeral'),
    isPoststartSidecar: hasClass('poststart-sidecar'),
    isPoststop: hasClass('poststop'),

    visit: clickable('a'),
  }),
};
