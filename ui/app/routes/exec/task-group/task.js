import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';

export default Route.extend({
  store: service(),

  afterModel(model) {
    // FIXME model isn’t a task, it’s just { task_name }… allocations!!
    this.controllerFor('exec').send('setTask', model);
  },
});
