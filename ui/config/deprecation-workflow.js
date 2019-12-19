/* global self */
self.deprecationWorkflow = self.deprecationWorkflow || {};
self.deprecationWorkflow.config = {
  workflow: [
    { handler: 'throw', matchId: 'ember-inflector.globals' },
    { handler: 'throw', matchId: 'ember-runtime.deprecate-copy-copyable' },
    { handler: 'throw', matchId: 'ember-console.deprecate-logger' },
    { handler: 'log', matchId: 'ember-test-helpers.rendering-context.jquery-element' },
  ],
};
