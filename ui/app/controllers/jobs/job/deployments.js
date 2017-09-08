import Ember from 'ember';

const { Controller, computed, inject } = Ember;

export default Controller.extend({
  jobController: inject.controller('jobs.job'),

  job: computed.alias('model'),
  deployments: computed.alias('model.deployments'),

  breadcrumbs: computed.alias('jobController.breadcrumbs'),
});
