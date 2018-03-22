import Controller, { inject as controller } from '@ember/controller';
import { alias } from '@ember/object/computed';

export default Controller.extend({
  taskController: controller('allocations.allocation.task'),
  breadcrumbs: alias('taskController.breadcrumbs'),
});
