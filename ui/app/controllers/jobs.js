import Controller from '@ember/controller';

export default class JobsController extends Controller {
  breadcrumbs = [
    {
      label: 'Jobs',
      args: ['jobs.index'],
    },
  ];
}
