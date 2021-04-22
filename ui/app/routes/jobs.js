import Route from '@ember/routing/route';
import classic from 'ember-classic-decorator';

@classic
export default class JobsRoute extends Route.extend() {
  breadcrumbs = [
    {
      label: 'Jobs',
      args: ['jobs.index'],
    },
  ];
}
