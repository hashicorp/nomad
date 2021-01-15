import Component from '@ember/component';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';
import { task } from 'ember-concurrency';
import { ForbiddenError } from '@ember-data/adapter/error';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';

@classic
@classNames('job-deployment', 'boxed-section')
export default class JobDeployment extends Component {
  deployment = null;
  isOpen = false;

  handleError() {}

  @task(function*() {
    try {
      yield this.deployment.fail();
    } catch (err) {
      // FIXME nothing actually handles errors at the moment
      let message = messageFromAdapterError(err);

      if (err instanceof ForbiddenError) {
        message = 'Your ACL token does not grant permission to fail deployments.';
      }

      this.handleError({
        title: 'Could Not Fail Deployment',
        description: message,
      });
    }
  })
  fail;
}
