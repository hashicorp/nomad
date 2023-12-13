/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* global self */
self.deprecationWorkflow = self.deprecationWorkflow || {};
self.deprecationWorkflow.config = {
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
    {
      handler: 'silence',
      matchId: 'implicit-injections',
    },
  ],
};
