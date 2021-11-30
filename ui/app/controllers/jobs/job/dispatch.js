import Controller from '@ember/controller';

export default class JobsJobDispatchController extends Controller {
  breadcrumbs = [
    {
      label: 'Dispatch',
      args: ['jobs.job.dispatch'],
    },
  ];
}
