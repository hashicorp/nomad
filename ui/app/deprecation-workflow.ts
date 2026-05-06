/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import setupDeprecationWorkflow from 'ember-cli-deprecation-workflow';

/**
 * Docs: https://github.com/ember-cli/ember-cli-deprecation-workflow
 */
setupDeprecationWorkflow({
  /**
    false by default, but if a developer / team wants to be more aggressive about being proactive with
    handling their deprecations, this should be set to "true"
  */
  throwOnUnhandled: false,
  workflow: [
    /* ... handlers ... */
    /* to generate this list, run your app for a while (or run the test suite),
     * and then run in the browser console:
     *
     *    deprecationWorkflow.flushDeprecations()
     *
     * And copy the handlers here
     */
    /* example: */
    /* { handler: 'silence', matchId: 'template-action' }, */
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
    { handler: 'silence', matchId: 'ember-data:deprecate-store-find' },
    { handler: 'silence', matchId: 'ember-basic-dropdown.config-environment' },
    {
      handler: 'silence',
      matchId: 'ember-data:deprecate-promise-many-array-behaviors',
    },
    { handler: 'silence', matchId: 'deprecate-array-prototype-extensions' },
    { handler: 'silence', matchId: 'ember-data:deprecate-model-reopenclass' },
    { handler: 'silence', matchId: 'ember-data:deprecate-early-static' },
    { handler: 'silence', matchId: 'ember-data:deprecate-array-like' },
    { handler: 'silence', matchId: 'deprecate-import-application-from-ember' },
    { handler: 'silence', matchId: 'deprecate-import-comparable-from-ember' },
    { handler: 'silence', matchId: 'deprecate-import-libraries-from-ember' },
    { handler: 'silence', matchId: 'deprecate-import-router-from-ember' },
    {
      handler: 'silence',
      matchId: 'deprecate-import--set-classic-decorator-from-ember',
    },
    { handler: 'silence', matchId: 'deprecate-import-meta-from-ember' },
    { handler: 'silence', matchId: 'importing-inject-from-ember-service' },
    { handler: 'silence', matchId: 'deprecate-import-testing-from-ember' },
    { handler: 'silence', matchId: 'deprecate-import-env-from-ember' },
  ],
});
