import Controller, { inject as controller } from '@ember/controller';
import { computed } from '@ember/object';

export default Controller.extend({
  allocationController: controller('allocations.allocation'),
  breadcrumbs: computed(
    'allocationController.breadcrumbs.[]',
    'model.{name,job,taskGroupName,shortId}',
    function() {
      return this.get('allocationController.breadcrumbs').concat([
        {
          label: this.get('model.name'),
          params: ['allocations.allocation.task', this.get('model.allocation'), this.get('model')],
        },
      ]);
    }
  ),
});
