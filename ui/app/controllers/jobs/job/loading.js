import Ember from 'ember';

const { Controller, computed, inject } = Ember;

export default Controller.extend({
  jobController: inject.controller('jobs.job'),
  breadcrumbs: computed.alias('jobController.breadcrumbs'),
});
