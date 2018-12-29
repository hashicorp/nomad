import Ember from 'ember';
import JSONFormatterPkg from 'npm:json-formatter-js';

const { Component, computed, run } = Ember;

// json-formatter-js is packaged in a funny way that ember-cli-browserify
// doesn't unwrap properly.
const { default: JSONFormatter } = JSONFormatterPkg;

export default Component.extend({
  classNames: ['json-viewer'],

  json: null,
  expandDepth: 2,

  formatter: computed('json', 'expandDepth', function() {
    return new JSONFormatter(this.get('json'), this.get('expandDepth'), {
      theme: 'nomad',
    });
  }),

  didReceiveAttrs() {
    const json = this.get('json');
    if (!json) {
      return;
    }

    run.scheduleOnce('afterRender', this, embedViewer);
  },
});

function embedViewer() {
  this.$().empty().append(this.get('formatter').render());
}
