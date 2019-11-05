import EmberObject, { computed } from '@ember/object';
import { later } from '@ember/runloop';

export default EmberObject.extend({
  init() {
    this._super(...arguments);
    this.set('complete', false);

    later(
      this,
      () => {
        this.set('complete', true);
      },
      100
    );
  },
});
