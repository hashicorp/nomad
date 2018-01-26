import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';

export default Model.extend({
  system: service(),

  name: attr('string'),
  address: attr('string'),
  serfPort: attr('string'),
  rpcPort: attr('string'),
  tags: attr({ defaultValue: () => ({}) }),
  status: attr('string'),
  datacenter: attr('string'),
  region: attr('string'),

  rpcAddr: computed('address', 'port', function() {
    const { address, rpcPort } = this.getProperties('address', 'rpcPort');
    return address && rpcPort && `${address}:${rpcPort}`;
  }),

  isLeader: computed('system.leader.rpcAddr', function() {
    return this.get('system.leader.rpcAddr') === this.get('rpcAddr');
  }),
});
