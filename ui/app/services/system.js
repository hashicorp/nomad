import Ember from 'ember';
import PromiseObject from '../utils/classes/promise-object';
import { namespace } from '../adapters/application';

const { Service, computed, inject } = Ember;

export default Service.extend({
  token: inject.service(),

  leader: computed(function() {
    const token = this.get('token');

    return PromiseObject.create({
      promise: token
        .authorizedRequest(`/${namespace}/status/leader`)
        .then(res => res.json())
        .then(rpcAddr => ({ rpcAddr }))
        .then(leader => {
          // Dirty self so leader can be used as a dependent key
          this.notifyPropertyChange('leader.rpcAddr');
          return leader;
        }),
    });
  }),
});
