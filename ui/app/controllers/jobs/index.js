import Ember from 'ember';

const { Controller, computed } = Ember;

export default Controller.extend({
  pendingJobs: computed.filterBy('model', 'status', 'pending'),
  runningJobs: computed.filterBy('model', 'status', 'running'),
  deadJobs: computed.filterBy('model', 'status', 'dead'),
});
