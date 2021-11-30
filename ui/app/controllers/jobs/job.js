import Controller from '@ember/controller';
import { jobCrumbs } from 'nomad-ui/utils/breadcrumb-utils';

export default class JobController extends Controller {
  queryParams = [
    {
      jobNamespace: 'namespace',
    },
  ];
  jobNamespace = 'default';

  breadcrumbs = jobCrumbs(this.model);
}
