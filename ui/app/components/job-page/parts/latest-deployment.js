import Component from '@ember/component';
import { task } from 'ember-concurrency';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('')
export default class LatestDeployment extends Component {
  job = null;

  handleError() {}

  isShowingDeploymentDetails = false;

  @task(function* () {
    try {
      yield this.get('job.latestDeployment.content').promote();
    } catch (err) {
      this.handleError({
        title: 'Could Not Promote Deployment',
        description: messageFromAdapterError(err, 'promote deployments'),
      });
    }
  })
  promote;

  @task(function* () {
    try {
      yield this.get('job.latestDeployment.content').fail();
    } catch (err) {
      this.handleError({
        title: 'Could Not Fail Deployment',
        description: messageFromAdapterError(err, 'fail deployments'),
      });
    }
  })
  fail;
}
