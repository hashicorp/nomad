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
    click: clickable(
      '.job-page-header .actions-dropdown .action-toggle-button'
    ),
    expandedValue: attribute(
      'aria-expanded',
      '.job-page-header .actions-dropdown .action-toggle-button'
    ),
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

  globalButton: {
    isPresent: isPresent('.actions-flyout-button button'),
    click: clickable('.actions-flyout-button button'),
  },

  flyout: {
    isPresent: isPresent('#actions-flyout'),
    instances: collection('.actions-queue .action-card', {
      text: text(),
      code: text('.messages code'),
      hasPeers: isPresent('.peers'),
      // titleBar: text('header'),
      statusBadge: text('header .hds-badge'),
    }),
    close: clickable('.hds-flyout__dismiss'),
    actions: {
      // find within actions-flyout
      isPresent: isPresent('#actions-flyout .actions-dropdown'),
      click: clickable('.actions-dropdown .action-toggle-button'),
      // expandedValue: attribute(
      //   'aria-expanded',
      //   '.actions-dropdown .action-toggle-button'
      // ),
      actions: collection('.actions-dropdown .hds-dropdown__list li', {
        text: text(),
        click: clickable('button'),
      }),
      multiAllocActions: collection(
        '.actions-dropdown .hds-dropdown__list li.hds-dropdown-list-item--variant-generic',
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
          showsDisclosureContent: isPresent(
            '.hds-disclosure-primitive__content'
          ),
        }
      ),
      singleAllocActions: collection(
        '.actions-dropdown .hds-dropdown__list li.hds-dropdown-list-item--variant-interactive',
        {
          text: text(),
          button: collection('button', {
            click: clickable(),
            expanded: attribute('aria-expanded'),
          }),
          showsDisclosureContent: isPresent(
            '.hds-disclosure-primitive__content'
          ),
        }
      ),
    },
  },
});
