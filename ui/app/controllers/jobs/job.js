import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { qpBuilder } from 'nomad-ui/utils/classes/query-params';

export default Controller.extend({
  breadcrumbs: computed('model.{name,id}', function() {
    return [
      { label: 'Jobs', args: ['jobs'] },
      {
        label: this.get('model.name'),
        args: [
          'jobs.job',
          this.get('model.plainId'),
          qpBuilder({
            jobNamespace: this.get('model.namespace.name') || 'default',
          }),
        ],
      },
    ];
  }),
});
