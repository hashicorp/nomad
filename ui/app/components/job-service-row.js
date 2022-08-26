import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class JobServiceRowComponent extends Component {
  @service router;

  @action
  gotoService(service) {
    if (service.provider === 'nomad') {
      this.router.transitionTo('jobs.job.services.service', service.name, {
        queryParams: { level: service.level },
        instances: service.instances,
      });
    }
  }
}
