import { alias } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';

export default Controller.extend({
  jobController: controller('jobs.job'),
  breadcrumbs: alias('jobController.breadcrumbs'),
});
