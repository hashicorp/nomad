import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class JobServiceRowComponent extends Component {
  @service router;

	@action
  gotoService(service) {
    this.router.transitionTo('jobs.job.services.service', service.name);
  }
}
