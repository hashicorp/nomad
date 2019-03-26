/* global self */
self.deprecationWorkflow = self.deprecationWorkflow || {};
self.deprecationWorkflow.config = {
  workflow: [
    { handler: 'throw', matchId: 'ember-inflector.globals' },
    { handler: 'silence', matchId: 'ember-runtime.deprecate-copy-copyable' },
    { handler: 'silence', matchId: 'ember-console.deprecate-logger' },
    { handler: 'silence', matchId: 'ember-component.send-action' },
  ],
};
