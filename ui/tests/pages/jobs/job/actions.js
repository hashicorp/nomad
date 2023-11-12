/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  attribute,
  clickable,
  create,
  collection,
  text,
  visitable,
  isPresent,
} from 'ember-cli-page-object';

export default create({
  visitIndex: visitable('/jobs/:id'),
  visitAllocs: visitable('/jobs/:id/allocations'),

  hasTitleActions: isPresent('.job-page-header .actions-dropdown'),

  taskRowActions: collection('td[data-test-actions] .actions-dropdown', {
    click: clickable('button'),
    actions: collection('.hds-dropdown__list li', {
      text: text(),
      click: clickable('button'),
    }),
  }),

  titleActions: {
    click: clickable('.job-page-header .actions-dropdown button'),
    actions: collection(
      '.job-page-header .actions-dropdown .hds-dropdown__list li',
      {
        text: text(),
        click: clickable('button'),
      }
    ),
    multiAllocActions: collection(
      '.job-page-header .actions-dropdown .hds-dropdown__list li.hds-dropdown-list-item--variant-generic',
      {
        text: text(),
        button: collection('button', {
          click: clickable(),
          expanded: attribute('aria-expanded'),
        }),
        subActions: collection(
          '.hds-disclosure-primitive__content .hds-reveal__content li',
          {
            text: text(),
            click: clickable('button'),
          }
        ),
        showsDisclosureContent: isPresent('.hds-disclosure-primitive__content'),
      }
    ),
    singleAllocActions: collection(
      '.job-page-header .actions-dropdown .hds-dropdown__list li.hds-dropdown-list-item--variant-interactive',
      {
        text: text(),
        button: collection('button', {
          click: clickable(),
          expanded: attribute('aria-expanded'),
        }),
        showsDisclosureContent: isPresent('.hds-disclosure-primitive__content'),
      }
    ),
  },

  toast: collection('.hds-toast', {
    text: text(),
    code: text('code'),
    titleBar: text('.hds-alert__title'),
  }),
});
