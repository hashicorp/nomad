import Mixin from '@ember/object/mixin';
import notifyError from 'nomad-ui/utils/notify-error';

export default Mixin.create({
  model() {
    return this._super(...arguments).catch(notifyError(this));
  },
});
