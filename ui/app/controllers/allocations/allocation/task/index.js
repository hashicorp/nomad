import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import { task } from 'ember-concurrency';

export default Controller.extend({
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

  error: computed(() => {
    // { title, description }
    return null;
  }),

  onDismiss() {
    this.set('error', null);
  },

  restartTask: task(function*() {
    try {
      yield this.model.restart();
    } catch (err) {
      this.set('error', {
        title: 'Could Not Restart Task',
        description: 'Your ACL token does not grant allocation lifecycle permissions.',
      });
    }
  }),
});
