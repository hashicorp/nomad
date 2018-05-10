import Component from '@ember/component';
import { computed } from '@ember/object';
import { inject as service } from '@ember/service';

export default Component.extend({
  store: service(),
  tagName: '',

  // When given a string, the component will fetch the allocation
  allocationId: null,

  // An allocation can also be provided directly
  allocation: computed('allocationId', function() {
    return this.get('store').findRecord('allocation', this.get('allocationId'));
  }),

  time: null,
  linkToAllocation: true,
  label: '',
});
