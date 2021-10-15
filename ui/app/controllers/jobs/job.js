import Controller from '@ember/controller';
import { tracked } from '@glimmer/tracking';

export default class JobController extends Controller {
  queryParams = [
    {
      jobNamespace: {
        as: 'namespace',
        refreshModel: true,
      },
    },
  ];

  @tracked jobNamespace = 'default';
}
