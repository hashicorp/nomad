import Mixin from '@ember/object/mixin';
import { run } from '@ember/runloop';
import { assert } from '@ember/debug';
import { on } from '@ember/object/evented';

// eslint-disable-next-line ember/no-new-mixins
export default Mixin.create({
  windowResizeHandler() {
    assert('windowResizeHandler needs to be overridden in the Component', false);
  },

  setupWindowResize: on('didInsertElement', function() {
    run.scheduleOnce('afterRender', this, this.addResizeListener);
  }),

  addResizeListener() {
    this.set('_windowResizeHandler', this.windowResizeHandler.bind(this));
    window.addEventListener('resize', this._windowResizeHandler);
  },

  removeWindowResize: on('willDestroyElement', function() {
    window.removeEventListener('resize', this._windowResizeHandler);
  }),
});
