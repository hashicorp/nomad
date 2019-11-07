import { A } from '@ember/array';
import ArrayProxy from '@ember/array/proxy';
import { next } from '@ember/runloop';

export default ArrayProxy.extend({
  init(array) {
    this.set('content', A([]));
    this._super(...arguments);

    next(this, () => {
      this.set('content', A(array));
    });
  },
});
