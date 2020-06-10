import Component from '@ember/component';
import { computed } from '@ember/object';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('')
export default class LifecycleChartRow extends Component {
  @computed('taskState.state')
  get activeClass() {
    if (this.taskState && this.taskState.state === 'running') {
      return 'is-active';
    }

    return;
  }

  @computed('taskState.finishedAt')
  get finishedClass() {
    if (this.taskState && this.taskState.finishedAt) {
      return 'is-finished';
    }

    return;
  }
}
