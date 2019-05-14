/* global self */
self.deprecationWorkflow = self.deprecationWorkflow || {};
self.deprecationWorkflow.config = {
  workflow: [
    { handler: 'throw', matchId: 'ember-inflector.globals' },
    { handler: 'throw', matchId: 'ember-runtime.deprecate-copy-copyable' },
    { handler: 'throw', matchId: 'ember-console.deprecate-logger' },
    // Only used in ivy-codemirror.
    // PR open: https://github.com/IvyApp/ivy-codemirror/pull/40/files
    { handler: 'log', matchId: 'ember-component.send-action' },
  ],
};
