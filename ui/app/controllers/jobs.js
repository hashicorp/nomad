import { inject as service } from '@ember/service';
import Controller from '@ember/controller';

export default class JobsController extends Controller {
  @service system;

  queryParams = [
    {
      jobNamespace: 'namespace',
    },
  ];

  isForbidden = false;

  jobNamespace = 'default';
}
