import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  tagName: '',

  model: null,

  allocation: computed('model', function() {
    if (this.model && this.model.allocation) {
      return this.model.allocation;
    } else {
      return this.model;
    }
  }),

  task: computed('model', function() {
    if (this.model && this.model.allocation) {
      return this.model;
    }
  }),
});
