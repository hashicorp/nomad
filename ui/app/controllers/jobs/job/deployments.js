import { alias } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';

export default Controller.extend(WithNamespaceResetting, {
  jobController: controller('jobs.job'),

  job: alias('model'),
  deployments: alias('model.deployments'),

  breadcrumbs: alias('jobController.breadcrumbs'),
});
