import Component from '@ember/component';
import { computed } from '@ember/object';
import { inject as service } from '@ember/service';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('boxed-section')
export default class JobClientSummary extends Component {
  @service store;

  job = null;
  jobClientStatus = null;

  @computed
  get isExpanded() {
    const storageValue = window.localStorage.nomadExpandJobClientSummary;
    return storageValue != null ? JSON.parse(storageValue) : true;
  }

  persist(item, isOpen) {
    window.localStorage.nomadExpandJobClientSummary = isOpen;
    this.notifyPropertyChange('isExpanded');
  }
}
