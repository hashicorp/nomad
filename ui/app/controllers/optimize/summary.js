import Controller from '@ember/controller';
import { inject as controller } from '@ember/controller';

export default class OptimizeSummaryController extends Controller {
  @controller('optimize') optimizeController;

  queryParams = [
    {
      jobNamespace: 'namespace',
    },
  ];

  get breadcrumbs() {
    const model = this.model;
    if (!model) return [];

    return [
      {
        label: model.slug.replace('/', ' / '),
        args: ['optimize.summary', model.slug],
      },
    ];
  }
}
