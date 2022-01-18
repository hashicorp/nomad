import Component from '@ember/component';
import { action, computed } from '@ember/object';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';
import jobClientStatus from 'nomad-ui/utils/properties/job-client-status';

@classic
@classNames('boxed-section')
export default class JobClientStatusSummary extends Component {
  job = null;
  nodes = null;
  forceCollapsed = false;
  gotoClients() {}

  @computed('forceCollapsed')
  get isExpanded() {
    if (this.forceCollapsed) return false;

    const storageValue = window.localStorage.nomadExpandJobClientStatusSummary;
    return storageValue != null ? JSON.parse(storageValue) : true;
  }

  @jobClientStatus('nodes', 'job') jobClientStatus;

  @action
  onSliceClick(ev, slice) {
    this.gotoClients([slice.className.camelize()]);
  }

  persist(item, isOpen) {
    window.localStorage.nomadExpandJobClientStatusSummary = isOpen;
    this.notifyPropertyChange('isExpanded');
  }
}
