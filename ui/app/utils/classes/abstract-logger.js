import Ember from 'ember';
import queryString from 'npm:query-string';

const { Mixin, computed, assign } = Ember;
const CHUNK_SIZE = 50000;

export default Mixin.create({
  url: '',
  params: computed(() => ({})),
  logFetch() {
    Ember.assert(
      'Loggers need a logFetch method, which should have an interface like window.fetch'
    );
  },

  endOffset: null,

  offsetParams: computed('endOffset', function() {
    const endOffset = this.get('endOffset');
    return endOffset
      ? { origin: 'start', offset: endOffset }
      : { origin: 'end', offset: CHUNK_SIZE };
  }),

  additionalParams: computed(() => ({})),

  fullUrl: computed('url', 'params', 'offsetParams', 'additionalParams', function() {
    const queryParams = queryString.stringify(
      assign({}, this.get('params'), this.get('offsetParams'), this.get('additionalParams'))
    );
    return `${this.get('url')}?${queryParams}`;
  }),
});
