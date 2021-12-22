import Component from '@ember/component';
import { action, computed } from '@ember/object';
import { inject as service } from '@ember/service';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('boxed-section')
export default class Summary extends Component {
  @service router;

  job = null;
  forceCollapsed = false;

  @action
  gotoAllocations(status) {
    this.router.transitionTo('jobs.job.allocations', this.job, {
      queryParams: {
        status: JSON.stringify(status),
        namespace: this.job.get('namespace.name'),
      },
    });
  }

  @action
  onSliceClick(ev, slice) {
    this.gotoAllocations([slice.label.camelize()]);
  }

  @computed('forceCollapsed')
  get isExpanded() {
    if (this.forceCollapsed) return false;

    const storageValue = window.localStorage.nomadExpandJobSummary;
    return storageValue != null ? JSON.parse(storageValue) : true;
  }

  persist(item, isOpen) {
    window.localStorage.nomadExpandJobSummary = isOpen;
    this.notifyPropertyChange('isExpanded');
  }
}
