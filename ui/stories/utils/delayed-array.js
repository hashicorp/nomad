import { A } from '@ember/array';
import ArrayProxy from '@ember/array/proxy';
import { later } from '@ember/runloop';

export default ArrayProxy.extend({
  init(array) {
    this.set('content', A([]));
    this._super(...arguments);

    later(
      this,
      () => {
        this.set('content', A(array));
      },
      100
    );
  },
});
