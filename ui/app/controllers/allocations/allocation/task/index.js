import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';

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
});
