import Controller from '@ember/controller';

export default class AllocationsAllocationTaskController extends Controller {
  get breadcrumbs() {
    const model = this.model;
    if (!model) return [];
    return [
      {
        label: model.get('name'),
        args: ['allocations.allocation.task', model.get('allocation'), model],
      },
    ];
  }
}
