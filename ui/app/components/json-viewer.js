import Component from '@ember/component';
import { computed } from '@ember/object';
import { run } from '@ember/runloop';
import { copy } from '@ember/object/internals';
import JSONFormatter from 'json-formatter-js';

export default Component.extend({
  classNames: ['json-viewer'],

  json: null,
  expandDepth: Infinity,

  formatter: computed('json', 'expandDepth', function() {
    return new JSONFormatter(copy(this.get('json'), true), this.get('expandDepth'), {
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
  this.$()
    .empty()
    .append(this.get('formatter').render());
}
