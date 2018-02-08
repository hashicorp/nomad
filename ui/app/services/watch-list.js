import { readOnly } from '@ember/object/computed';
import { copy } from '@ember/object/internals';
import Service from '@ember/service';

const list = {};

export default Service.extend({
  list: readOnly(function() {
    return copy(list, true);
  }),

  getIndexFor(url) {
    return list[url] || 0;
  },

  setIndexFor(url, value) {
    list[url] = value;
  },
});
