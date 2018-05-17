import { assert } from '@ember/debug';
import Mixin from '@ember/object/mixin';
import { computed } from '@ember/object';
import { assign } from '@ember/polyfills';
import queryString from 'npm:query-string';

const MAX_OUTPUT_LENGTH = 50000;

export default Mixin.create({
  url: '',
  params: computed(() => ({})),
  logFetch() {
    assert('Loggers need a logFetch method, which should have an interface like window.fetch');
  },

  endOffset: null,

  offsetParams: computed('endOffset', function() {
    const endOffset = this.get('endOffset');
    return endOffset
      ? { origin: 'start', offset: endOffset }
      : { origin: 'end', offset: MAX_OUTPUT_LENGTH };
  }),

  additionalParams: computed(() => ({})),

  fullUrl: computed('url', 'params', 'offsetParams', 'additionalParams', function() {
    const queryParams = queryString.stringify(
      assign({}, this.get('params'), this.get('offsetParams'), this.get('additionalParams'))
    );
    return `${this.get('url')}?${queryParams}`;
  }),
});
