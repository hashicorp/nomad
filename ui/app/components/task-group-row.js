import Component from '@ember/component';
import { lazyClick } from '../helpers/lazy-click';
import { classNames, tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('tr')
@classNames('task-group-row', 'is-interactive')
export default class TaskGroupRow extends Component {
  taskGroup = null;

  onClick() {}

  click(event) {
    lazyClick([this.onClick, event]);
  }
}
