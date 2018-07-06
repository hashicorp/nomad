import { alias } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';

export default Controller.extend(WithNamespaceResetting, {
  jobController: controller('jobs.job'),

  job: alias('model'),
  evaluations: alias('model.evaluations'),

  breadcrumbs: alias('jobController.breadcrumbs'),
});
