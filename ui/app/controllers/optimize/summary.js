import Controller from '@ember/controller';
import { inject as controller } from '@ember/controller';

export default class OptimizeSummaryController extends Controller {
  @controller('optimize') optimizeController;

  queryParams = [
    {
      jobNamespace: 'namespace',
    },
  ];
}
