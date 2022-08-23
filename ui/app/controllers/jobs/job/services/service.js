import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class JobsJobServicesServiceController extends Controller {
  @service router;

	@action
  gotoAllocation(allocation) {
		console.log('alloc', allocation);
    this.router.transitionTo('allocations.allocation', allocation.get('id'));
  }

}
