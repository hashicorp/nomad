import Controller from '@ember/controller';
import { computed } from '@ember/object';
import queryParams from 'nomad-ui/utils/queryParams';

export default Controller.extend({
  breadcrumbs: computed('model', 'model.{name,id}', 'model.namespace.name', function() {
    return [
      {
        label: 'Jobs',
        params: ['jobs'],
      },
      {
        label: this.get('model.name'),
        params: [
          'jobs.job',
          this.get('model'),
          queryParams({
            jobNamespace: this.get('model.namespace.name'),
          }),
        ],
      },
    ];
  }),
});
