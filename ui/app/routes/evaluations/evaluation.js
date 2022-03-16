import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';

export default class EvaluationRoute extends Route {
  @service store;

  model(params) {
    return this.store.findRecord('evaluation', params.evaluation_id);
  }
}
