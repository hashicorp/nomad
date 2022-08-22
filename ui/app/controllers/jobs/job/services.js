import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { alias } from '@ember/object/computed';

export default class JobsJobServicesController extends Controller.extend(
  WithNamespaceResetting
) {
  @alias('model') job;

  // @alias('job.services') services;

  // Services, grouped by name, with aggregatable allocations.
	get services() {
    return this.job.services.reduce((m,n) => {
			let siblingServiceInstance = m.findBy('name', n.name);
			if (!siblingServiceInstance) {
				siblingServiceInstance = n;
				m.push(n);
			}
			siblingServiceInstance.allocations = [...(siblingServiceInstance.allocations || []), n.allocation];
			return m;
    }, [])
  }
}
