import Component from '@ember/component';
import { action, computed } from '@ember/object';
import { inject as service } from '@ember/service';
import PromiseArray from 'nomad-ui/utils/classes/promise-array';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('boxed-section')
export default class RecentAllocations extends Component {
  @service router;

  sortProperty = 'modifyIndex';
  sortDescending = true;

  @computed('job.allocations.@each.modifyIndex')
  get sortedAllocations() {
    return PromiseArray.create({
      promise: this.get('job.allocations').then(allocations =>
        allocations
          .sortBy('modifyIndex')
          .reverse()
          .slice(0, 5)
      ),
    });
  }

  @action
  gotoAllocation(allocation) {
    this.router.transitionTo('allocations.allocation', allocation.id);
  }
}
