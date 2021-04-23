import Controller from '@ember/controller';

export default class JobController extends Controller {
  queryParams = [
    {
      jobNamespace: 'namespace',
    },
  ];
  jobNamespace = 'default';
}
