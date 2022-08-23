import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { alias } from '@ember/object/computed';
import { computed, get } from '@ember/object';

export default class JobsJobServicesIndexController extends Controller.extend(
  WithNamespaceResetting
) {
  @alias('model') job;

  // Services, grouped by name, with aggregatable allocations.
  @computed('job.services.@each.{name,allocation}', 'job.services.length', 'job.services')
  get services() {
    return this.job.services.reduce((m, n) => {
      n.allocations = [];
      let siblingServiceInstance = m.findBy('name', n.name);
      if (!siblingServiceInstance) {
        siblingServiceInstance = n;
        m.push(n);
      }
      siblingServiceInstance.allocations = [
        ...(siblingServiceInstance.allocations || []),
        n.allocation,
      ];
      return m;
    }, []);
  }
}
