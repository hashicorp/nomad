/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  clickable,
  collection,
  create,
  hasClass,
  isPresent,
  text,
  triggerable,
  visitable,
} from 'ember-cli-page-object';

export default create({
  visitJob: visitable('/exec/:job'),
  visitTaskGroup: visitable('/exec/:job/:task_group'),
  visitTask: visitable('/exec/:job/:task_group/:task_name'),

  header: {
    region: { scope: '[data-test-region]' },
    namespace: { scope: '[data-test-namespace]' },
    job: text('[data-test-job]'),
  },

  taskGroups: collection('[data-test-task-group]', {
    click: clickable('[data-test-task-group-name]'),
    name: text('[data-test-task-group-name]'),

    isLoading: hasClass('is-loading', '[data-test-task-group-name]'),

    chevron: {
      scope: '.toggle-button .icon',
      isDown: hasClass('icon-is-chevron-down'),
      isRight: hasClass('icon-is-chevron-right'),
    },

    tasks: collection('[data-test-task]', {
      name: text(),
      isActive: isPresent('[data-test-task-active]'),
    }),
  }),

  terminal: {
    scope: '.xterm-helper-textarea',
    pressEnter: triggerable('keydown', '', {
      eventProperties: { keyCode: 13 },
    }),
  },

  jobDead: {
    scope: '[data-test-exec-job-dead]',
    message: text('[data-test-exec-job-dead-message]'),
  },
});
