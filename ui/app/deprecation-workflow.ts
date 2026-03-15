/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import setupDeprecationWorkflow from 'ember-cli-deprecation-workflow';

setupDeprecationWorkflow({
  workflow: [
    { handler: 'throw', matchId: 'ember-inflector.globals' },
    { handler: 'throw', matchId: 'ember-runtime.deprecate-copy-copyable' },
    { handler: 'throw', matchId: 'ember-console.deprecate-logger' },
    {
      handler: 'throw',
      matchId: 'ember-test-helpers.rendering-context.jquery-element',
    },
    { handler: 'throw', matchId: 'ember-cli-page-object.is-property' },
    { handler: 'throw', matchId: 'ember-views.partial' },
    { handler: 'silence', matchId: 'ember-string.prototype-extensions' },
    {
      handler: 'silence',
      matchId: 'ember-glimmer.link-to.positional-arguments',
    },
    { handler: 'silence', matchId: 'implicit-injections' },
    { handler: 'silence', matchId: 'template-action' },
    {
      handler: 'silence',
      matchId: 'ember-concurrency.deprecate-classic-task-api',
    },
    {
      handler: 'silence',
      matchId: 'ember-concurrency.deprecate-decorator-task',
    },
    {
      handler: 'silence',
      matchId: 'ember-data:deprecate-store-find',
    },
    {
      handler: 'silence',
      matchId: 'ember-basic-dropdown.config-environment',
    },
  ],
});
