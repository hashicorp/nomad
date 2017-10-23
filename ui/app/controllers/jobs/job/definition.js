import Ember from 'ember';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';

const { Controller, computed, inject } = Ember;

export default Controller.extend(WithNamespaceResetting, {
  jobController: inject.controller('jobs.job'),

  job: computed.alias('model.job'),

  breadcrumbs: computed.alias('jobController.breadcrumbs'),
});
