import Component from '@ember/component';
import { computed } from '@ember/object';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('boxed-section')
export default class Summary extends Component {
  job = null;

  @computed
  get isExpanded() {
    const storageValue = window.localStorage.nomadExpandJobSummary;
    return storageValue != null ? JSON.parse(storageValue) : true;
  }

  persist(item, isOpen) {
    window.localStorage.nomadExpandJobSummary = isOpen;
    this.notifyPropertyChange('isExpanded');
  }
}
