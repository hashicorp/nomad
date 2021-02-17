import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import Model from '@ember-data/model';
import { attr } from '@ember-data/model';
import classic from 'ember-classic-decorator';

@classic
export default class Agent extends Model {
  @service system;

  @attr('string') name;
  @attr('string') address;
  @attr('string') serfPort;
  @attr('string') rpcPort;
  @attr({ defaultValue: () => ({}) }) tags;
  @attr('string') status;
  @attr('string') datacenter;
  @attr('string') region;

  @computed('address', 'port')
  get rpcAddr() {
    const { address, rpcPort } = this;
    return address && rpcPort && `${address}:${rpcPort}`;
  }

  @computed('rpcAddr', 'system.leader.rpcAddr')
  get isLeader() {
    return this.get('system.leader.rpcAddr') === this.rpcAddr;
  }
}
