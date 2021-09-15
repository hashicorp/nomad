import { inject as service } from '@ember/service';
import Component from '@ember/component';
import { lazyClick } from '../helpers/lazy-click';
import { classNames, tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('tr')
@classNames('job-row', 'is-interactive')
export default class JobRow extends Component {
  @service system;
  @service store;

  job = null;

  // One of independent, parent, or child. Used to customize the template
  // based on the relationship of this job to others.
  context = 'independent';

  onClick() {}

  click(event) {
    lazyClick([this.onClick, event]);
  }
}
