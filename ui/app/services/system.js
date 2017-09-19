import Ember from 'ember';
import fetch from 'fetch';
import PromiseObject from '../utils/classes/promise-object';
import { namespace } from '../adapters/application';

const { Service, computed } = Ember;

export default Service.extend({
  leader: computed(function() {
    return PromiseObject.create({
      promise: fetch(`/${namespace}/status/leader`)
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
