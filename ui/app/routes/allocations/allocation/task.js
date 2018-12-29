import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import EmberError from '@ember/error';

export default Route.extend({
  store: service(),

  breadcrumbs(model) {
    if (!model) return [];
    return [
      {
        label: model.get('name'),
        args: ['allocations.allocation.task', model.get('allocation'), model],
      },
    ];
  },

  model({ name }) {
    const allocation = this.modelFor('allocations.allocation');

    // If there is no allocation, then there is no task.
    // Let the allocation route handle the 404 error.
    if (!allocation) return;

    const task = allocation.get('states').findBy('name', name);

    if (!task) {
      const err = new EmberError(`Task ${name} not found for allocation ${allocation.get('id')}`);
      err.code = '404';
      this.controllerFor('application').set('error', err);
    }

    return task;
  },
});
