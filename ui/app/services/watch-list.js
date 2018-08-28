import { readOnly } from '@ember/object/computed';
import { copy } from '@ember/object/internals';
import Service from '@ember/service';

let list = {};

export default Service.extend({
  list: readOnly(function() {
    return copy(list, true);
  }),

  init() {
    list = {};
  },

  getIndexFor(url) {
    return list[url] || 1;
  },

  setIndexFor(url, value) {
    list[url] = +value;
  },
});
