import Component from '@ember/component';
import { action, computed } from '@ember/object';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('boxed-section')
export default class JobClientStatusSummary extends Component {
  job = null;
  jobClientStatus = null;
  gotoClients() {}

  @computed
  get isExpanded() {
    const storageValue = window.localStorage.nomadExpandJobClientStatusSummary;
    return storageValue != null ? JSON.parse(storageValue) : true;
  }

  @action
  onSliceClick(slice) {
    this.gotoClients([slice.className.camelize()]);
  }

  persist(item, isOpen) {
    window.localStorage.nomadExpandJobClientStatusSummary = isOpen;
    this.notifyPropertyChange('isExpanded');
  }
}
