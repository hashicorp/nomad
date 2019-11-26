import Component from '@ember/component';
import { computed as overridable } from 'ember-overridable-computed';
import { inject as service } from '@ember/service';

export default Component.extend({
  store: service(),
  tagName: '',

  // When given a string, the component will fetch the allocation
  allocationId: null,

  // An allocation can also be provided directly
  allocation: overridable('allocationId', function() {
    return this.store.findRecord('allocation', this.allocationId);
  }),

  time: null,
  linkToAllocation: true,
  label: '',
});
