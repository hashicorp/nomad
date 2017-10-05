import Ember from 'ember';
import notifyError from 'nomad-ui/utils/notify-error';

const { Mixin } = Ember;

export default Mixin.create({
  model() {
    return this._super(...arguments).catch(notifyError(this));
  },
});
