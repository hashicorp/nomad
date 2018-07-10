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
    if (allocation) {
      const task = allocation.get('states').findBy('name', name);

      if (task) {
        return task;
      }

      const err = new EmberError(`Task ${name} not found for allocation ${allocation.get('id')}`);
      err.code = '404';
      this.controllerFor('application').set('error', err);
    }
  },
});
