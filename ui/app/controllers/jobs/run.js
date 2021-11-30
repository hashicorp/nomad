import Controller from '@ember/controller';

export default class RunController extends Controller {
  breadcrumbs = [
    {
      label: 'Run',
      args: ['jobs.run'],
    },
  ];

  onSubmit(id, namespace) {
    this.transitionToRoute('jobs.job', id, {
      queryParams: { namespace },
    });
  }
}
