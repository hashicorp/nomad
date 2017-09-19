import Ember from 'ember';

const { Controller, computed, inject } = Ember;

export default Controller.extend({
  jobController: inject.controller('jobs.job'),

  job: computed.alias('model'),
  versions: computed.alias('model.versions'),

  breadcrumbs: computed.alias('jobController.breadcrumbs'),
});
