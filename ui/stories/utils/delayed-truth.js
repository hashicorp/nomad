import EmberObject from '@ember/object';
import { next } from '@ember/runloop';

export default EmberObject.extend({
  init() {
    this._super(...arguments);
    this.set('complete', false);

    next(this, () => {
      this.set('complete', true);
    });
  },
});
