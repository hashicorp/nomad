import Component from '@ember/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { lazyClick } from '../helpers/lazy-click';
import { classNames, tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('tr')
@classNames('job-row', 'is-interactive')
export default class JobRow extends Component {
  @service router;
  @service store;
  @service system;

  job = null;

  // One of independent, parent, or child. Used to customize the template
  // based on the relationship of this job to others.
  context = 'independent';

  click(event) {
    lazyClick([this.gotoJob, event]);
  }

  @action
  gotoJob() {
    const { job } = this;
    this.router.transitionTo('jobs.job', job.plainId, {
      queryParams: { namespace: job.get('namespace.name') },
    });
  }
}
