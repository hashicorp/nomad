import Route from '@ember/routing/route';

export default class JobsJobServicesRoute extends Route {
  async model() {
    console.log('calling the model for', this.modelFor('jobs'), this.modelFor('jobs.job'));
    return this.modelFor('jobs.job');
  }
}
