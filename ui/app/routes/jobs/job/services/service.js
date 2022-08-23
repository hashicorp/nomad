import Route from '@ember/routing/route';

export default class JobsJobServicesServiceRoute extends Route {
	model({name}) {
		const services = this.modelFor('jobs.job').get('services').filterBy('name', name);
		return services;
	}
}
