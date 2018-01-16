import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import { computed } from '@ember/object';

export default Controller.extend({
  breadcrumbs: computed(
    'model.{name,allocation}',
    'model.allocation.{job,taskGroupName}',
    'model.allocation.job.name',
    function() {
      return [
        {
          label: 'Jobs',
          args: ['jobs'],
        },
        {
          label: this.get('model.allocation.job.name'),
          args: ['jobs.job', this.get('model.allocation.job')],
        },
        {
          label: this.get('model.allocation.taskGroupName'),
          args: [
            'jobs.job.task-group',
            this.get('model.allocation.job'),
            this.get('model.allocation.taskGroupName'),
          ],
        },
        {
          label: this.get('model.allocation.shortId'),
          args: ['allocations.allocation', this.get('model.allocation')],
        },
        {
          label: this.get('model.name'),
          args: ['allocations.allocation.task', this.get('model.allocation'), this.get('model')],
        },
      ];
    }
  ),
  network: alias('model.resources.networks.firstObject'),
  ports: computed('network.reservedPorts.[]', 'network.dynamicPorts.[]', function() {
    return (this.get('network.reservedPorts') || [])
      .map(port => ({
        name: port.Label,
        port: port.Value,
        isDynamic: false,
      }))
      .concat(
        (this.get('network.dynamicPorts') || []).map(port => ({
          name: port.Label,
          port: port.Value,
          isDynamic: true,
        }))
      )
      .sortBy('name');
  }),
});
