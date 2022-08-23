import Route from '@ember/routing/route';

export default class JobsJobServicesServiceRoute extends Route {
  beforeModel(transition) {
    console.log('transition', transition);
    // transition.to.set('queryParams', {});
    transition.to.queryParams = {}; // TODO: queryParams not being reset
  }
  model(params) {
    console.log('params', params);
    const { name } = params;
    const services = this.modelFor('jobs.job')
      .get('services')
      .filterBy('name', name);
    return services;
  }
}
